package ingest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExtractNewsContentSuccessMarkdownAndWordCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header was empty")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := fmt.Fprint(w, `<!doctype html>
<html>
<head>
	<title>Fallback Article Title</title>
</head>
<body>
	<nav>Skip navigation</nav>
	<article>
		<h1>Readable Headline</h1>
		<p>Alpha beta gamma delta.</p>
		<p><strong>Markets</strong> moved after the policy update.</p>
	</article>
</body>
</html>`); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	result := extractNewsContent(context.Background(), server.URL, server.Client())

	if result.ExtractionError != "" {
		t.Fatalf("ExtractionError = %q, want empty", result.ExtractionError)
	}
	if result.ExtractedTitle == nil || strings.TrimSpace(*result.ExtractedTitle) == "" {
		t.Fatalf("ExtractedTitle = %v, want populated title", result.ExtractedTitle)
	}
	if result.ExtractedContent == nil {
		t.Fatal("ExtractedContent = nil, want markdown content")
	}
	if !strings.Contains(*result.ExtractedContent, "Alpha beta gamma delta.") {
		t.Fatalf("ExtractedContent = %q, want article paragraph", *result.ExtractedContent)
	}
	if !strings.Contains(*result.ExtractedContent, "**Markets**") {
		t.Fatalf("ExtractedContent = %q, want markdown bold formatting", *result.ExtractedContent)
	}
	if result.ExtractedWordCount == nil || *result.ExtractedWordCount < 9 {
		t.Fatalf("ExtractedWordCount = %v, want at least article word count", result.ExtractedWordCount)
	}
}

func TestParseNewsContentHTMLUsesOGTitleFallback(t *testing.T) {
	pageURL := mustParseURLForTest(t, "https://example.test/story")
	body := []byte(`<!doctype html>
<html>
<head>
	<meta property="og:title" content="  OG Fallback Title  ">
</head>
<body>
	<article>
		<p>One two three four five six seven eight nine ten eleven twelve.</p>
	</article>
</body>
</html>`)

	result := parseNewsContentHTML(body, pageURL)

	if result.ExtractionError != "" {
		t.Fatalf("ExtractionError = %q, want empty", result.ExtractionError)
	}
	if result.ExtractedTitle == nil {
		t.Fatal("ExtractedTitle = nil, want OG fallback title")
	}
	if *result.ExtractedTitle != "OG Fallback Title" {
		t.Fatalf("ExtractedTitle = %q, want %q", *result.ExtractedTitle, "OG Fallback Title")
	}
}

func TestExtractNewsContentNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no article here", http.StatusForbidden)
	}))
	defer server.Close()

	result := extractNewsContent(context.Background(), server.URL, server.Client())

	if !strings.Contains(result.ExtractionError, "unexpected HTTP status 403") {
		t.Fatalf("ExtractionError = %q, want HTTP status error", result.ExtractionError)
	}
	if result.ExtractedTitle != nil || result.ExtractedContent != nil || result.ExtractedWordCount != nil {
		t.Fatalf("result = %+v, want no extracted fields", result)
	}
}

func TestEnrichNewsArticleBucketsDedupesDuplicateLinks(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := fmt.Fprint(w, `<!doctype html>
<html>
<head><title>Shared Article</title></head>
<body>
	<article>
		<h1>Shared Article</h1>
		<p>This shared article should be fetched only once by the extraction layer.</p>
	</article>
</body>
</html>`); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	categoryBuckets := []NewsArticleBucket{
		{
			ID:   "technology_ai",
			Name: "Technology & AI",
			Articles: []NewsArticle{
				{Title: "Shared", Link: server.URL},
			},
		},
	}
	regionBuckets := []NewsArticleBucket{
		{
			ID:   "us",
			Name: "U.S.",
			Articles: []NewsArticle{
				{Title: "Shared duplicate", Link: strings.ToUpper(server.URL)},
			},
		},
	}

	categoryBuckets, regionBuckets = enrichNewsArticleBucketsWithContent(context.Background(), categoryBuckets, regionBuckets)

	if got := requestCount.Load(); got != 1 {
		t.Fatalf("requestCount = %d, want 1", got)
	}
	categoryArticle := categoryBuckets[0].Articles[0]
	regionArticle := regionBuckets[0].Articles[0]
	if categoryArticle.ExtractionError != "" || regionArticle.ExtractionError != "" {
		t.Fatalf("ExtractionError category=%q region=%q, want empty", categoryArticle.ExtractionError, regionArticle.ExtractionError)
	}
	if categoryArticle.ExtractedContent == nil || regionArticle.ExtractedContent == nil {
		t.Fatalf("ExtractedContent category=%v region=%v, want both populated", categoryArticle.ExtractedContent, regionArticle.ExtractedContent)
	}
	if *categoryArticle.ExtractedContent != *regionArticle.ExtractedContent {
		t.Fatalf("duplicate articles got different extracted content: %q vs %q", *categoryArticle.ExtractedContent, *regionArticle.ExtractedContent)
	}
}

func mustParseURLForTest(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) returned error: %v", rawURL, err)
	}
	return parsedURL
}
