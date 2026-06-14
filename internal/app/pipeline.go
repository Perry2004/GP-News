package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Perry2004/GP-News/briefing"
	"github.com/Perry2004/GP-News/ingest"
	"github.com/Perry2004/GP-News/internal/history"
)

type generatedBriefing struct {
	Email       briefing.BriefingEmail
	FinalInput  briefing.BriefingInput
	SourceInput briefing.BriefingAgentInput
}

func generateBriefing(ctx context.Context, cfg config, freshFrom FreshFrom) (generatedBriefing, error) {
	if freshFrom == FreshFromCached {
		slog.Info("Loading cached final briefing", "cache_dir", cfg.CacheDir)
		briefingEmail, err := briefing.LoadCachedBriefingEmail(cfg.CacheDir)
		if err != nil {
			return generatedBriefing{}, fmt.Errorf("failed to load cached final briefing: %w", err)
		}
		return generatedBriefing{Email: briefingEmail}, nil
	}

	llm, err := briefing.NewLLMGenerator(briefing.Config{
		BaseURL:             cfg.BaseURL,
		APIKey:              cfg.LLMAPIKey,
		Model:               cfg.Model,
		MaxCompletionTokens: cfg.LLMMaxCompletionTokens,
		Temperature:         cfg.LLMTemperature,
		ThinkingLevel:       cfg.LLMThinkingLevel,
		ProviderIgnore:      cfg.LLMProviderIgnore,
		PersistData:         cfg.PersistData,
		CacheDir:            cfg.CacheDir,
	})
	if err != nil {
		return generatedBriefing{}, fmt.Errorf("failed to create LLM generator: %w", err)
	}

	var input briefing.BriefingAgentInput // input is only needed when new input to the final generation is changed.
	if freshFromNeedsRawData(freshFrom) {
		input, err = buildBriefingAgentInput(ctx, cfg, freshFrom)
		if err != nil {
			return generatedBriefing{}, err
		}
		input = applyBriefingHistoryDedupe(ctx, cfg, freshFrom, llm, input)
	}

	var result briefing.GenerationResult
	switch freshFrom {
	case FreshFromFetching, FreshFromSummarization:
		result, err = llm.GenerateBriefingResult(ctx, input)
	case FreshFromReview:
		processed, loadErr := briefing.LoadCachedProcessedNews(cfg.CacheDir)
		if loadErr != nil {
			return generatedBriefing{}, fmt.Errorf("failed to load cached processed news: %w", loadErr)
		}
		result, err = llm.GenerateBriefingFromProcessedNewsResult(ctx, input, processed)
	case FreshFromBriefing:
		finalInput, loadErr := briefing.LoadCachedFinalBriefingInput(cfg.CacheDir)
		if loadErr != nil {
			return generatedBriefing{}, fmt.Errorf("failed to load cached final briefing input: %w", loadErr)
		}
		email, generateErr := llm.GenerateFinalBriefing(ctx, finalInput)
		err = generateErr
		result = briefing.GenerationResult{Email: email, FinalInput: finalInput}
	default:
		return generatedBriefing{}, fmt.Errorf("invalid FRESH_FROM %q", cfg.FreshFrom)
	}
	if err != nil {
		return generatedBriefing{}, fmt.Errorf("failed to generate briefing: %w", err)
	}
	slog.Info("Generated briefing", "output", result.Email)
	return generatedBriefing{Email: result.Email, FinalInput: result.FinalInput, SourceInput: input}, nil
}

func applyBriefingHistoryDedupe(ctx context.Context, cfg config, freshFrom FreshFrom, llm *briefing.LLMGenerator, input briefing.BriefingAgentInput) briefing.BriefingAgentInput {
	if !briefingHistoryEnabled(cfg, freshFrom) {
		return input
	}

	store, err := history.NewDynamoDBStore(ctx, cfg.BriefingHistoryTable)
	if err != nil {
		slog.Warn("Briefing history dedupe disabled after store setup failure", "error", err)
		return input
	}
	since := time.Now().AddDate(0, 0, -cfg.BriefingHistoryLookbackDays)
	records, err := store.RecentSelectedNews(ctx, since)
	if err != nil {
		slog.Warn("Briefing history dedupe skipped after DynamoDB query failure", "error", err)
		return input
	}
	if len(records) == 0 {
		slog.Info("Briefing history dedupe skipped; no recent selected news found")
		return input
	}

	result, err := llm.FilterBriefingHistoryDuplicates(ctx, input, history.DedupeRecentNews(records))
	if err != nil {
		slog.Warn("Briefing history dedupe skipped after LLM failure", "error", err)
		return input
	}
	if len(result.DuplicateArticleIDs) > 0 {
		slog.Info("Briefing history dedupe removed duplicate articles",
			"duplicate_article_ids", result.DuplicateArticleIDs,
			"original_article_count", len(input.Articles),
			"filtered_article_count", len(result.Input.Articles),
		)
	}
	return result.Input
}

func persistSelectedBriefingHistory(ctx context.Context, cfg config, freshFrom FreshFrom, generated generatedBriefing) {
	if !briefingHistoryEnabled(cfg, freshFrom) {
		return
	}
	if len(generated.FinalInput.ReviewedNews) == 0 {
		slog.Info("Briefing history write skipped; no reviewed news selected")
		return
	}

	store, err := history.NewDynamoDBStore(ctx, cfg.BriefingHistoryTable)
	if err != nil {
		slog.Warn("Briefing history write skipped after store setup failure", "error", err)
		return
	}
	now := time.Now()
	runID := now.UTC().Format("20060102T150405Z")
	records := history.BuildSelectedNewsRecords(generated.FinalInput, generated.SourceInput.Articles, runID, now, cfg.BriefingHistoryTTLDays)
	if err := store.PutSelectedNews(ctx, records); err != nil {
		slog.Warn("Briefing history write failed", "error", err, "record_count", len(records))
		return
	}
	slog.Info("Briefing history written", "record_count", len(records), "table", cfg.BriefingHistoryTable)
}

func briefingHistoryEnabled(cfg config, freshFrom FreshFrom) bool {
	if strings.TrimSpace(cfg.BriefingHistoryTable) == "" {
		return false
	}
	switch freshFrom {
	case FreshFromFetching, FreshFromSummarization:
		return true
	default:
		return false
	}
}

func buildBriefingAgentInput(ctx context.Context, cfg config, freshFrom FreshFrom) (briefing.BriefingAgentInput, error) {
	marketValues, categoryBuckets, regionBuckets, err := ingest.RetrieveData(ctx, ingest.Config{
		NewsDataAPIKey: cfg.NewsDataAPIKey,
		EnableFetching: freshFrom == FreshFromFetching,
		PersistData:    cfg.PersistData,
		CacheDir:       cfg.CacheDir,
	})
	if err != nil {
		return briefing.BriefingAgentInput{}, err
	}

	now := time.Now()
	input := briefing.BriefingAgentInput{
		BriefingDate:   now.Format(time.DateOnly),
		Session:        briefingSession(now),
		MarketSnapshot: buildMarketInputs(marketValues),
		Articles:       buildArticleInputs(categoryBuckets, regionBuckets),
	}
	if cfg.PersistData {
		if err := briefing.StoreExtractedNews(cfg.CacheDir, input.Articles); err != nil {
			slog.Error("Failed to store extracted news cache JSON", "error", err)
		} else {
			slog.Info("Extracted news cache JSON stored", "cache_dir", cfg.CacheDir, "article_count", len(input.Articles))
		}
	}
	return input, nil
}

// Returns a list of articles with valid links, de-duplicated by link.
func buildArticleInputs(categoryBuckets []ingest.NewsArticleBucket, regionBuckets []ingest.NewsArticleBucket) []briefing.ArticleInput {
	articles := make([]briefing.ArticleInput, 0)
	seenLinks := make(map[string]bool)

	addBucket := func(bucket ingest.NewsArticleBucket) {
		for i, article := range bucket.Articles {
			linkKey := strings.TrimSpace(article.Link)
			if linkKey != "" {
				if seenLinks[linkKey] {
					continue
				}
				seenLinks[linkKey] = true
			}

			articles = append(articles, briefing.ArticleInput{
				ID:                 fmt.Sprintf("%s_%d", bucket.ID, i),
				BucketID:           bucket.ID,
				BucketName:         bucket.Name,
				Title:              article.Title,
				Link:               article.Link,
				ExtractedTitle:     optionalStringValue(article.ExtractedTitle),
				ExtractedContent:   optionalStringValue(article.ExtractedContent),
				ExtractedWordCount: optionalIntValue(article.ExtractedWordCount),
				ExtractionError:    article.ExtractionError,
			})
		}
	}

	for _, bucket := range categoryBuckets {
		addBucket(bucket)
	}
	for _, bucket := range regionBuckets {
		addBucket(bucket)
	}

	return articles
}

func buildMarketInputs(marketValues []ingest.MarketValue) []briefing.MarketInput {
	marketInputs := make([]briefing.MarketInput, 0, len(marketValues))
	for _, value := range marketValues {
		marketInputs = append(marketInputs, briefing.MarketInput{
			ID:          value.ID,
			Name:        value.Name,
			Category:    value.Category,
			Symbol:      value.Symbol,
			Level:       fmt.Sprintf("%g", value.Value),
			DailyChange: formatMarketDailyChange(value),
			Timestamp:   value.Timestamp.Format(time.RFC3339),
			History:     buildMarketHistoryInputs(value.History),
			Source:      value.Source,
		})
	}
	return marketInputs
}

func formatMarketDailyChange(value ingest.MarketValue) string {
	if !value.DailyChangeValid {
		return ""
	}
	return fmt.Sprintf("%+.2f (%+.2f%%)", value.DailyChange, value.DailyChangePercent)
}

func buildMarketHistoryInputs(history []ingest.MarketHistoryPoint) []briefing.MarketHistoryPoint {
	if len(history) == 0 {
		return nil
	}

	points := make([]briefing.MarketHistoryPoint, 0, len(history))
	for _, point := range history {
		points = append(points, briefing.MarketHistoryPoint{
			Timestamp: point.Timestamp.Format(time.RFC3339),
			Close:     fmt.Sprintf("%g", point.Close),
		})
	}
	return points
}

// Returns Morning or Night based on the time.
func briefingSession(now time.Time) string {
	if now.Hour() < 12 {
		return "Morning"
	}
	return "Night"
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
