package briefing

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

const (
	historyDedupeContentExcerptMaxChars = 1500
	historyDedupeMaxConcurrentCalls     = 50
)

type BriefingHistoryDedupeRecentNews struct {
	HistoryEntryID       string   `json:"history_entry_id"`
	BriefingDate         string   `json:"briefing_date"`
	Session              string   `json:"session"`
	SourceURL            string   `json:"source_url"`
	SourceName           string   `json:"source_name"`
	ProcessedHeadline    string   `json:"processed_headline"`
	Summary              string   `json:"summary"`
	Entities             []string `json:"entities"`
	Region               string   `json:"region"`
	AssetClasses         []string `json:"asset_classes"`
	WhyItMatters         string   `json:"why_it_matters"`
	PossibleMarketImpact string   `json:"possible_market_impact"`
	ReviewNote           string   `json:"review_note"`
}

type BriefingHistoryDedupeCurrentArticle struct {
	ArticleID      string `json:"article_id"`
	BucketID       string `json:"bucket_id"`
	BucketName     string `json:"bucket_name"`
	Title          string `json:"title"`
	ExtractedTitle string `json:"extracted_title"`
	Link           string `json:"link"`
	ContentExcerpt string `json:"content_excerpt"`
}

type BriefingHistoryDedupeInput struct {
	BriefingDate       string                              `json:"briefing_date"`
	Session            string                              `json:"session"`
	CurrentArticle     BriefingHistoryDedupeCurrentArticle `json:"current_article"`
	RecentSelectedNews []BriefingHistoryDedupeRecentNews   `json:"recent_selected_news"`
}

type BriefingHistoryDedupeMatch struct {
	HistoryEntryID string `json:"history_entry_id"`
	Confidence     string `json:"confidence" jsonschema:"enum=High,enum=Medium,enum=Low"`
	Reason         string `json:"reason"`
}

type BriefingHistoryDedupeOutput struct {
	Duplicate bool                         `json:"duplicate"`
	Matches   []BriefingHistoryDedupeMatch `json:"matches"`
}

type BriefingHistoryDedupeResult struct {
	Input               BriefingAgentInput
	Outputs             map[string]BriefingHistoryDedupeOutput
	DuplicateArticleIDs []string
}

type historyDedupeArticleResult struct {
	index  int
	id     string
	output BriefingHistoryDedupeOutput
	err    error
}

func (g *LLMGenerator) FilterBriefingHistoryDuplicates(ctx context.Context, input BriefingAgentInput, recent []BriefingHistoryDedupeRecentNews) (BriefingHistoryDedupeResult, error) {
	result := BriefingHistoryDedupeResult{Input: input, Outputs: map[string]BriefingHistoryDedupeOutput{}}
	if len(input.Articles) == 0 || len(recent) == 0 {
		return result, nil
	}

	candidates := historyDedupeCurrentArticleCandidates(input.Articles)
	if len(candidates) == 0 {
		return result, nil
	}

	results := make(chan historyDedupeArticleResult, len(candidates))
	limiter := make(chan struct{}, historyDedupeMaxConcurrentCalls)
	var wg sync.WaitGroup
	for _, candidate := range candidates {
		wg.Add(1)
		go func(candidate historyDedupeArticleResult) {
			defer wg.Done()
			select {
			case limiter <- struct{}{}:
				defer func() { <-limiter }()
			case <-ctx.Done():
				results <- historyDedupeArticleResult{index: candidate.index, id: candidate.id, err: ctx.Err()}
				return
			}

			output, err := g.generateBriefingHistoryDedupe(ctx, BriefingHistoryDedupeInput{
				BriefingDate:       input.BriefingDate,
				Session:            input.Session,
				CurrentArticle:     historyDedupeCurrentArticle(input.Articles[candidate.index]),
				RecentSelectedNews: recent,
			})
			results <- historyDedupeArticleResult{index: candidate.index, id: candidate.id, output: output, err: err}
		}(candidate)
	}
	wg.Wait()
	close(results)

	duplicateIDsByIndex := make([]string, len(input.Articles))
	for articleResult := range results {
		if articleResult.err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			slog.Warn("Keeping article after failed briefing history dedupe",
				"article_id", articleResult.id,
				"error", articleResult.err,
			)
			continue
		}
		result.Outputs[articleResult.id] = articleResult.output
		if articleResult.output.Duplicate {
			duplicateIDsByIndex[articleResult.index] = articleResult.id
		}
	}

	duplicateIDs := compactNonEmptyStrings(duplicateIDsByIndex)
	filtered, duplicateIDs := filterDuplicateArticleIDs(input.Articles, duplicateIDs)
	input.Articles = filtered
	result.Input = input
	result.DuplicateArticleIDs = duplicateIDs
	slog.Info("Briefing history semantic dedupe completed",
		"recent_selected_count", len(recent),
		"candidate_article_count", len(candidates),
		"duplicate_article_count", len(duplicateIDs),
	)
	return result, nil
}

func (g *LLMGenerator) generateBriefingHistoryDedupe(ctx context.Context, input BriefingHistoryDedupeInput) (BriefingHistoryDedupeOutput, error) {
	var output BriefingHistoryDedupeOutput
	err := generateStructured(ctx, g, structuredRequest[BriefingHistoryDedupeOutput]{
		Name:        "briefing_history_dedupe",
		Description: "Semantic duplicate decision for one current briefing article against recent selected briefing history.",
		Schema:      briefingHistoryDedupeOutputSchema(),
		System:      briefingHistoryDedupeSystemPrompt(),
		Input:       input,
		Output:      &output,
		LogAttrs:    []any{"article_id", input.CurrentArticle.ArticleID},
	})
	if err != nil {
		return BriefingHistoryDedupeOutput{}, err
	}
	return output, nil
}

func historyDedupeCurrentArticleCandidates(articles []ArticleInput) []historyDedupeArticleResult {
	candidates := make([]historyDedupeArticleResult, 0, len(articles))
	for i, article := range articles {
		if strings.TrimSpace(article.ExtractionError) != "" {
			continue
		}
		candidates = append(candidates, historyDedupeArticleResult{index: i, id: article.ID})
	}
	return candidates
}

func historyDedupeCurrentArticle(article ArticleInput) BriefingHistoryDedupeCurrentArticle {
	return BriefingHistoryDedupeCurrentArticle{
		ArticleID:      article.ID,
		BucketID:       article.BucketID,
		BucketName:     article.BucketName,
		Title:          article.Title,
		ExtractedTitle: article.ExtractedTitle,
		Link:           article.Link,
		ContentExcerpt: truncateRunes(strings.TrimSpace(article.ExtractedContent), historyDedupeContentExcerptMaxChars),
	}
}

func compactNonEmptyStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		compacted = append(compacted, value)
	}
	return compacted
}

func filterDuplicateArticleIDs(articles []ArticleInput, duplicateIDs []string) ([]ArticleInput, []string) {
	available := make(map[string]bool, len(articles))
	for _, article := range articles {
		available[article.ID] = true
	}

	duplicates := make(map[string]bool, len(duplicateIDs))
	filteredDuplicateIDs := make([]string, 0, len(duplicateIDs))
	for _, id := range duplicateIDs {
		id = strings.TrimSpace(id)
		if id == "" || !available[id] || duplicates[id] {
			continue
		}
		duplicates[id] = true
		filteredDuplicateIDs = append(filteredDuplicateIDs, id)
	}
	if len(duplicates) == 0 {
		return articles, nil
	}

	filtered := make([]ArticleInput, 0, len(articles)-len(duplicates))
	for _, article := range articles {
		if duplicates[article.ID] {
			continue
		}
		filtered = append(filtered, article)
	}
	return filtered, filteredDuplicateIDs
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func briefingHistoryDedupeOutputSchema() any {
	return schemaFor[BriefingHistoryDedupeOutput]()
}

func briefingHistoryDedupeSystemPrompt() string {
	return fmt.Sprintf(`You are the GP News semantic briefing-history dedupe stage.
Return only the requested JSON. Compare the single current article against recent selected briefing news.
Set duplicate=true only when the current article covers the same underlying event, announcement, market-moving development, or materially identical update that was already selected in a recent briefing.
Treat cross-source reports of the same development as duplicates.
Do not mark an article as a duplicate merely because it shares a broad topic, company, country, sector, or continuing theme.
If the article is a meaningful new development, escalation, market reaction, fresh data point, or follow-up with new facts, set duplicate=false.
When uncertain, set duplicate=false.
Every match history_entry_id must be from recent_selected_news.
Reasons must be concise and explain the same underlying development.
All natural-language fields must be English.
%s`, englishOutputPrompt)
}
