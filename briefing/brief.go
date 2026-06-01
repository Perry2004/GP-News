package briefing

import (
	"context"
	"fmt"
	"log/slog"
)

const (
	finalNewsCardMin = 5
	finalNewsCardMax = 15
)

func (g *LLMGenerator) generateFinalBriefing(ctx context.Context, state *briefingAgentState) (BriefingEmail, error) {
	input := BriefingInput{
		BriefingDate:   state.input.BriefingDate,
		Session:        state.input.Session,
		MarketSnapshot: state.input.MarketSnapshot,
		ReviewedNews:   state.reviewedNews(),
		ReviewSummary:  state.reviewSummary,
		ThemeClusters:  nil,
	}
	slog.Debug("Generating final briefing",
		"briefing_date", input.BriefingDate,
		"session", input.Session,
		"market_item_count", len(input.MarketSnapshot),
		"reviewed_news_count", len(input.ReviewedNews),
	)

	output, err := g.generateFinalBriefingWithPrompt(ctx, input, briefingSystemPrompt())
	if err != nil {
		return BriefingEmail{}, err
	}
	if err := validateFinalNewsCardCount(output); err == nil {
		return output, nil
	}

	invalidCount := finalNewsCardCount(output)
	slog.Warn("Final briefing news card count invalid; retrying final generation",
		"news_card_count", invalidCount,
		"min_news_cards", finalNewsCardMin,
		"max_news_cards", finalNewsCardMax,
	)
	output, err = g.generateFinalBriefingWithPrompt(ctx, input, briefingRetrySystemPrompt(invalidCount))
	if err != nil {
		return BriefingEmail{}, err
	}
	if err := validateFinalNewsCardCount(output); err != nil {
		return BriefingEmail{}, err
	}
	return output, nil
}

func (g *LLMGenerator) generateFinalBriefingWithPrompt(ctx context.Context, input BriefingInput, systemPrompt string) (BriefingEmail, error) {
	var output BriefingEmail
	err := generateStructured(ctx, g, structuredRequest[BriefingEmail]{
		Name:        "briefing_email",
		Description: "Email-ready GP News briefing JSON.",
		Schema:      briefingEmailSchema(),
		System:      systemPrompt,
		Input:       input,
		Output:      &output,
	})
	return output, err
}

func validateFinalNewsCardCount(briefing BriefingEmail) error {
	count := finalNewsCardCount(briefing)
	if count < finalNewsCardMin || count > finalNewsCardMax {
		return fmt.Errorf("final briefing has %d total full news cards across top_news_by_topic; want %d to %d", count, finalNewsCardMin, finalNewsCardMax)
	}
	return nil
}

func finalNewsCardCount(briefing BriefingEmail) int {
	return len(briefing.TopNewsByTopic.MarketsMacro) +
		len(briefing.TopNewsByTopic.PoliticsPolicy) +
		len(briefing.TopNewsByTopic.WarGeopoliticalRisk) +
		len(briefing.TopNewsByTopic.TechnologyAI)
}

func briefingSystemPrompt() string {
	return fmt.Sprintf(`You are the GP News Intelligence Desk composer.
Return only the requested JSON. Build a compact market briefing for an investor/operator audience.
Follow these rules:
- exactly 3 read_this_first items
- 5 to 15 total full news cards across top_news_by_topic when enough reviewed news exists
- total full news cards means len(markets_macro) + len(politics_policy) + len(war_geopolitical_risk) + len(technology_ai)
- never exceed 15 total full news cards
- any top_news_by_topic category may be empty when it has no strong reviewed news
- 5 to 8 regional_radar items when enough processed news exists
- 2 to 3 watch_next items
- every market item must include asset, level, daily_change, timestamp, driver, and source
- never claim a daily move unless daily_change is present in supplied market data
- source every major news card from supplied reviewed news only
- use review corrections and additional context when they clarify or supersede first-pass processed summaries
- use review_summary for global framing, but do not mention the review process itself
- keep paragraphs short and avoid generic filler
- no "what changed since last briefing" section
For briefing output, all prose fields, headlines, radar sentences, watch items, drivers, and reasons must be English.
%s`, englishOutputPrompt)
}

func briefingRetrySystemPrompt(invalidCount int) string {
	return fmt.Sprintf(`%s

Your previous final briefing had %d total full news cards across top_news_by_topic.
That violates the required %d to %d total full news card range.
Return corrected JSON with %d to %d total full news cards across markets_macro, politics_policy, war_geopolitical_risk, and technology_ai combined.`,
		briefingSystemPrompt(),
		invalidCount,
		finalNewsCardMin,
		finalNewsCardMax,
		finalNewsCardMin,
		finalNewsCardMax,
	)
}
