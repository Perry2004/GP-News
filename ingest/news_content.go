package ingest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	readability "github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
)

const (
	defaultNewsContentMaxConcurrentCalls = 50
	newsContentBatchTimeout              = 1 * time.Minute
	newsContentFetchTimeout              = 20 * time.Second
	newsContentBodyLimit                 = 5 << 20
)

type newsContentExtractionResult struct {
	ExtractedTitle     *string
	ExtractedContent   *string
	ExtractedWordCount *int
	ExtractionError    string
}

type newsContentJobResult struct {
	normalizedLink string
	result         newsContentExtractionResult
}

type newsContentLimiter chan struct{}

func enrichNewsArticleBucketsWithContent(ctx context.Context, categoryBuckets []NewsArticleBucket, regionBuckets []NewsArticleBucket) ([]NewsArticleBucket, []NewsArticleBucket) {
	totalArticles := countNewsArticles(categoryBuckets) + countNewsArticles(regionBuckets)
	if totalArticles == 0 {
		slog.Info("Skipping news article content extraction; no articles found")
		return categoryBuckets, regionBuckets
	}

	extractCtx, cancel := context.WithTimeout(ctx, newsContentBatchTimeout)
	defer cancel()

	links := collectUniqueNewsArticleLinks(categoryBuckets, regionBuckets)
	if len(links) == 0 {
		slog.Info("Skipping news article content extraction; no article links found", "article_count", totalArticles)
		return categoryBuckets, regionBuckets
	}

	slog.Info("Starting news article content extraction",
		"unique_link_count", len(links),
		"article_count", totalArticles,
		"max_concurrent_calls", defaultNewsContentMaxConcurrentCalls,
		"batch_timeout", newsContentBatchTimeout.String(),
		"article_timeout", newsContentFetchTimeout.String(),
	)

	startedAt := time.Now()
	client := newNewsContentHTTPClient()
	results := extractNewsContents(extractCtx, links, client, defaultNewsContentMaxConcurrentCalls)
	categoryBuckets = applyNewsContentExtractionResults(categoryBuckets, results)
	regionBuckets = applyNewsContentExtractionResults(regionBuckets, results)

	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.ExtractionError != "" {
			failureCount++
			continue
		}
		successCount++
	}
	slog.Info("News article content extraction completed",
		"unique_link_count", len(links),
		"article_count", totalArticles,
		"success_count", successCount,
		"failure_count", failureCount,
		"duration", time.Since(startedAt).String(),
	)

	return categoryBuckets, regionBuckets
}

func collectUniqueNewsArticleLinks(bucketGroups ...[]NewsArticleBucket) []string {
	links := make([]string, 0)
	seen := make(map[string]bool)
	for _, buckets := range bucketGroups {
		for _, bucket := range buckets {
			for _, article := range bucket.Articles {
				normalizedLink := normalizeNewsArticleLink(article.Link)
				if normalizedLink == "" || seen[normalizedLink] {
					continue
				}
				seen[normalizedLink] = true
				links = append(links, article.Link)
			}
		}
	}
	return links
}

func extractNewsContents(ctx context.Context, links []string, client *http.Client, maxConcurrentCalls int) map[string]newsContentExtractionResult {
	if maxConcurrentCalls <= 0 {
		maxConcurrentCalls = defaultNewsContentMaxConcurrentCalls
	}

	slog.Debug("Dispatching news article content extraction jobs",
		"unique_link_count", len(links),
		"max_concurrent_calls", maxConcurrentCalls,
	)

	results := make(map[string]newsContentExtractionResult, len(links))
	resultsChan := make(chan newsContentJobResult, len(links))
	limiter := make(newsContentLimiter, maxConcurrentCalls)

	for _, link := range links {
		go func(link string) {
			normalizedLink := normalizeNewsArticleLink(link)
			if normalizedLink == "" {
				slog.Warn("Skipping news article content extraction for empty URL")
				resultsChan <- newsContentJobResult{
					normalizedLink: normalizedLink,
					result: newsContentExtractionResult{
						ExtractionError: "empty article URL",
					},
				}
				return
			}

			if err := limiter.acquire(ctx); err != nil {
				slog.Warn("News article content extraction skipped before request",
					"url", link,
					"error", truncateLogValue(err.Error(), 500),
				)
				resultsChan <- newsContentJobResult{
					normalizedLink: normalizedLink,
					result: newsContentExtractionResult{
						ExtractionError: err.Error(),
					},
				}
				return
			}
			defer limiter.release()

			startedAt := time.Now()
			slog.Debug("Extracting news article content", "url", link)
			result := extractNewsContent(ctx, link, client)
			duration := time.Since(startedAt)
			if result.ExtractionError != "" {
				slog.Warn("News article content extraction failed",
					"url", link,
					"duration", duration.String(),
					"error", truncateLogValue(result.ExtractionError, 500),
				)
			} else {
				extractedTitle := ""
				if result.ExtractedTitle != nil {
					extractedTitle = truncateLogValue(*result.ExtractedTitle, 200)
				}
				extractedWordCount := 0
				if result.ExtractedWordCount != nil {
					extractedWordCount = *result.ExtractedWordCount
				}
				contentBytes := 0
				if result.ExtractedContent != nil {
					contentBytes = len(*result.ExtractedContent)
				}
				slog.Debug("News article content extracted",
					"url", link,
					"duration", duration.String(),
					"extracted_title", extractedTitle,
					"extracted_word_count", extractedWordCount,
					"content_bytes", contentBytes,
				)
			}

			resultsChan <- newsContentJobResult{
				normalizedLink: normalizedLink,
				result:         result,
			}
		}(link)
	}

	for range links {
		result := <-resultsChan
		if result.normalizedLink == "" {
			continue
		}
		results[result.normalizedLink] = result.result
	}

	return results
}

func applyNewsContentExtractionResults(buckets []NewsArticleBucket, results map[string]newsContentExtractionResult) []NewsArticleBucket {
	appliedCount := 0
	for bucketIndex := range buckets {
		for articleIndex := range buckets[bucketIndex].Articles {
			normalizedLink := normalizeNewsArticleLink(buckets[bucketIndex].Articles[articleIndex].Link)
			result, ok := results[normalizedLink]
			if !ok {
				continue
			}
			buckets[bucketIndex].Articles[articleIndex].ExtractedTitle = result.ExtractedTitle
			buckets[bucketIndex].Articles[articleIndex].ExtractedContent = result.ExtractedContent
			buckets[bucketIndex].Articles[articleIndex].ExtractedWordCount = result.ExtractedWordCount
			buckets[bucketIndex].Articles[articleIndex].ExtractionError = result.ExtractionError
			appliedCount++
		}
	}
	slog.Debug("Applied news article content extraction results",
		"bucket_count", len(buckets),
		"article_count", countNewsArticles(buckets),
		"applied_count", appliedCount,
	)
	return buckets
}

func extractNewsContent(ctx context.Context, articleURL string, client *http.Client) newsContentExtractionResult {
	parsedURL, err := url.Parse(strings.TrimSpace(articleURL))
	if err != nil {
		return newsContentExtractionResult{ExtractionError: fmt.Errorf("parse article URL: %w", err).Error()}
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return newsContentExtractionResult{ExtractionError: fmt.Sprintf("invalid article URL %q", articleURL)}
	}

	requestCtx, cancel := context.WithTimeout(ctx, newsContentFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return newsContentExtractionResult{ExtractionError: fmt.Errorf("build request: %w", err).Error()}
	}
	setNewsContentRequestHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return newsContentExtractionResult{ExtractionError: fmt.Errorf("request failed: %w", err).Error()}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, newsContentBodyLimit))
	if err != nil {
		return newsContentExtractionResult{ExtractionError: fmt.Errorf("read response: %w", err).Error()}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newsContentExtractionResult{ExtractionError: fmt.Sprintf("unexpected HTTP status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}

	return parseNewsContentHTML(body, parsedURL)
}

func parseNewsContentHTML(body []byte, pageURL *url.URL) newsContentExtractionResult {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return newsContentExtractionResult{ExtractionError: fmt.Errorf("parse HTML: %w", err).Error()}
	}

	fallbackTitle := fallbackNewsContentTitle(doc)
	article, err := readability.FromDocument(doc, pageURL)
	if err != nil {
		result := newsContentExtractionResult{ExtractionError: fmt.Errorf("extract readable content: %w", err).Error()}
		if fallbackTitle != nil {
			result.ExtractedTitle = fallbackTitle
		}
		return result
	}

	var extractedTitle *string
	if title := strings.TrimSpace(article.Title); title != "" {
		extractedTitle = &title
	} else {
		extractedTitle = fallbackTitle
	}

	var extractedContent *string
	if strings.TrimSpace(article.Content) != "" {
		markdown, err := htmltomarkdown.ConvertString(article.Content, converter.WithDomain(pageURL.String()))
		if err != nil {
			return newsContentExtractionResult{
				ExtractedTitle:  extractedTitle,
				ExtractionError: fmt.Errorf("convert extracted content to markdown: %w", err).Error(),
			}
		}
		if markdown = strings.TrimSpace(markdown); markdown != "" {
			extractedContent = &markdown
		}
	}

	var extractedWordCount *int
	if wordCount := len(strings.Fields(article.TextContent)); wordCount > 0 {
		extractedWordCount = &wordCount
	}

	return newsContentExtractionResult{
		ExtractedTitle:     extractedTitle,
		ExtractedContent:   extractedContent,
		ExtractedWordCount: extractedWordCount,
	}
}

func newNewsContentHTTPClient() *http.Client {
	client := NewHTTPClient()
	client.Timeout = newsContentFetchTimeout
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		setNewsContentRequestHeaders(req)
		return nil
	}
	return client
}

func setNewsContentRequestHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
}

func fallbackNewsContentTitle(doc *html.Node) *string {
	if title := findMetaPropertyContent(doc, "og:title"); title != "" {
		return &title
	}
	if title := strings.TrimSpace(findHTMLTitle(doc)); title != "" {
		return &title
	}
	return nil
}

func findMetaPropertyContent(node *html.Node, property string) string {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "meta") {
		var propertyValue string
		var contentValue string
		for _, attr := range node.Attr {
			switch strings.ToLower(attr.Key) {
			case "property":
				propertyValue = strings.TrimSpace(attr.Val)
			case "content":
				contentValue = strings.TrimSpace(attr.Val)
			}
		}
		if propertyValue == property && contentValue != "" {
			return contentValue
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if content := findMetaPropertyContent(child, property); content != "" {
			return content
		}
	}
	return ""
}

func findHTMLTitle(node *html.Node) string {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "title") {
		return strings.TrimSpace(nodeTextContent(node))
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if title := findHTMLTitle(child); title != "" {
			return title
		}
	}
	return ""
}

func nodeTextContent(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func normalizeNewsArticleLink(link string) string {
	return strings.TrimSpace(strings.ToLower(link))
}

func truncateLogValue(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "..."
}

func (limiter newsContentLimiter) acquire(ctx context.Context) error {
	select {
	case limiter <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (limiter newsContentLimiter) release() {
	<-limiter
}
