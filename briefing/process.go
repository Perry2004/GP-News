package briefing

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

const processNewsMaxConcurrentCalls = 20

type processNewsResult struct {
	index int
	news  ProcessedNews
	err   error
}

func (g *LLMGenerator) GenerateProcessedNews(ctx context.Context, input ProcessNewsInput) (ProcessedNews, error) {
	var output ProcessedNews
	slog.Debug("Processing news",
		"briefing_date", input.BriefingDate,
		"article_id", input.Article.ID,
	)
	err := generateStructured(ctx, g, structuredRequest[ProcessedNews]{
		Name:        "processed_news",
		Description: "Processed single news entry for a market briefing.",
		Schema:      processedNewsSchema(),
		System:      processedNewsSystemPrompt(),
		Input:       input,
		Output:      &output,
		LogAttrs:    []any{"article_id", input.Article.ID},
	})
	if err != nil {
		return ProcessedNews{}, err
	}
	slog.Debug("Processed news",
		"briefing_date", input.BriefingDate,
		"article_id", input.Article.ID,
		"headline", output.Headline,
	)
	output.ArticleID = input.Article.ID
	return output, nil
}

// Filters out articles with extraction errors.
func filterValidArticles(articles []ArticleInput) []ArticleInput {
	valid := make([]ArticleInput, 0, len(articles))
	for _, article := range articles {
		if strings.TrimSpace(article.ExtractionError) != "" {
			continue
		}
		valid = append(valid, article)
	}
	return valid
}

func (g *LLMGenerator) processNewsConcurrently(ctx context.Context, briefingDate string, articles []ArticleInput) ([]ProcessedNews, error) {
	if len(articles) == 0 {
		return nil, nil
	}

	results := make(chan processNewsResult, len(articles))
	limiter := make(chan struct{}, processNewsMaxConcurrentCalls)
	var wg sync.WaitGroup
	for i, article := range articles {
		wg.Add(1)
		go func(index int, article ArticleInput) {
			defer wg.Done()
			select {
			case limiter <- struct{}{}:
				defer func() { <-limiter }()
			case <-ctx.Done():
				results <- processNewsResult{index: index, err: ctx.Err()}
				return
			}
			news, err := g.GenerateProcessedNews(ctx, ProcessNewsInput{
				BriefingDate: briefingDate,
				Article:      article,
			})
			results <- processNewsResult{index: index, news: news, err: err}
		}(i, article)
	}
	wg.Wait()
	close(results)

	ordered := make([]ProcessedNews, len(articles))
	included := make([]bool, len(articles))
	skippedCount := 0
	for result := range results {
		if result.err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, fmt.Errorf("process news %q: %w", articles[result.index].ID, ctxErr)
			}
			skippedCount++
			slog.Warn("Excluding article after failed news processing",
				"article_id", articles[result.index].ID,
				"error", result.err,
			)
			continue
		}
		ordered[result.index] = result.news
		included[result.index] = true
	}

	processed := make([]ProcessedNews, 0, len(articles)-skippedCount)
	for i, news := range ordered {
		if included[i] {
			processed = append(processed, news)
		}
	}
	if skippedCount > 0 {
		slog.Warn("Excluded failed news processing entries",
			"processed_count", len(processed),
			"skipped_count", skippedCount,
			"valid_article_count", len(articles),
		)
	}
	return processed, nil
}

func processedNewsArticleIDSet(processed []ProcessedNews) map[string]bool {
	ids := make(map[string]bool, len(processed))
	for _, news := range processed {
		id := strings.TrimSpace(news.ArticleID)
		if id == "" {
			continue
		}
		ids[id] = true
	}
	return ids
}

func filterArticlesByIDSet(articles []ArticleInput, ids map[string]bool) []ArticleInput {
	filtered := make([]ArticleInput, 0, len(articles))
	for _, article := range articles {
		if ids[article.ID] {
			filtered = append(filtered, article)
		}
	}
	return filtered
}

func processedNewsSystemPrompt() string {
	return fmt.Sprintf(`You are the single-news processing stage for an investor/operator daily briefing.
Return only the requested JSON. For the supplied article, summarize, analyze, determine market relevance, and decide whether it should be included.
Do not invent source names, market moves, dates, or unsupported claims. Use low confidence for thin or failed extractions.
Scores must be 0 to 10. Confidence must be High, Medium, or Low.
For processed news, headline, summary, region, why_it_matters, and possible_market_impact must be English.
For entities, use common English names when available; preserve personal names and official organization names when appropriate.
%s`, englishOutputPrompt)
}
