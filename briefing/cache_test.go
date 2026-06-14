package briefing

import "testing"

func TestLoadCachedBriefingEmailReadsFinalOutputCache(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	want := BriefingEmail{
		Subject:          "Cached final briefing",
		CriticalityScore: 7.5,
		PriorityLevel:    "Important",
	}
	if err := writeCacheJSON(cachedBriefingFilePath(cacheDir, finalBriefingOutputCacheFileName), want); err != nil {
		t.Fatalf("write cache JSON: %v", err)
	}

	got, err := LoadCachedBriefingEmail(cacheDir)
	if err != nil {
		t.Fatalf("LoadCachedBriefingEmail() error = %v", err)
	}
	if got.Subject != want.Subject || got.CriticalityScore != want.CriticalityScore || got.PriorityLevel != want.PriorityLevel {
		t.Fatalf("cached briefing = %#v, want %#v", got, want)
	}
}

func TestLoadCachedProcessedNewsReadsProcessedCache(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	want := []ProcessedNews{
		{
			ArticleID:            "a1",
			Headline:             "Cached headline",
			Summary:              "Cached summary.",
			MarketRelevanceScore: 8,
			KeepForBriefing:      true,
			SourceURL:            "https://example.test/a1",
		},
	}
	if err := writeCacheJSON(cachedBriefingFilePath(cacheDir, processedNewsCacheFileName), want); err != nil {
		t.Fatalf("write cache JSON: %v", err)
	}

	got, err := LoadCachedProcessedNews(cacheDir)
	if err != nil {
		t.Fatalf("LoadCachedProcessedNews() error = %v", err)
	}
	if len(got) != 1 || got[0].ArticleID != want[0].ArticleID || got[0].Headline != want[0].Headline {
		t.Fatalf("cached processed news = %#v, want %#v", got, want)
	}
}

func TestStoreExtractedNewsWritesArticleInputCache(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	want := []ArticleInput{
		{
			ID:                 "markets_macro_0",
			BucketID:           "markets_macro",
			BucketName:         "Markets & Macro",
			Title:              "Extracted title",
			Link:               "https://example.test/news",
			ExtractedTitle:     "Full extracted title",
			ExtractedContent:   "Extracted but unprocessed article body.",
			ExtractedWordCount: 6,
		},
	}
	if err := StoreExtractedNews(cacheDir, want); err != nil {
		t.Fatalf("StoreExtractedNews() error = %v", err)
	}

	var got []ArticleInput
	if err := readCacheJSON(cachedBriefingFilePath(cacheDir, extractedNewsCacheFileName), &got); err != nil {
		t.Fatalf("read extracted news cache: %v", err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID || got[0].ExtractedContent != want[0].ExtractedContent {
		t.Fatalf("cached extracted news = %#v, want %#v", got, want)
	}
}

func TestLoadCachedFinalBriefingInputReadsReviewedInputCache(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	want := BriefingInput{
		BriefingDate: "2026-05-30",
		Session:      "Morning",
		ReviewedNews: []ReviewedNews{
			{News: ProcessedNews{ArticleID: "a1", Headline: "Cached headline"}, PriorityScore: 8.5},
		},
		ReviewSummary: ReviewSummary{SelectionRationale: "Selected cached item."},
	}
	if err := writeCacheJSON(cachedBriefingFilePath(cacheDir, finalBriefingInputCacheFileName), want); err != nil {
		t.Fatalf("write cache JSON: %v", err)
	}

	got, err := LoadCachedFinalBriefingInput(cacheDir)
	if err != nil {
		t.Fatalf("LoadCachedFinalBriefingInput() error = %v", err)
	}
	if got.BriefingDate != want.BriefingDate || len(got.ReviewedNews) != 1 || got.ReviewedNews[0].News.ArticleID != "a1" {
		t.Fatalf("cached final briefing input = %#v, want %#v", got, want)
	}
}
