package briefing

import (
	"context"
	"fmt"
	"log/slog"
)

func (g *LLMGenerator) generateFinalBriefing(ctx context.Context, state *briefingAgentState) (BriefingEmail, error) {
	var output BriefingEmail
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
	err := generateStructured(ctx, g, structuredRequest[BriefingEmail]{
		Name:        "briefing_email",
		Description: "Email-ready GP News briefing JSON.",
		Schema:      briefingEmailSchema(),
		System:      briefingSystemPrompt(),
		Input:       input,
		Output:      &output,
	})
	return output, err
}

func briefingSystemPrompt() string {
	return fmt.Sprintf(`You are the GP News Intelligence Desk composer.
Return only the requested JSON. Build a compact market briefing for an investor/operator audience.
Follow these rules:
- exactly 3 read_this_first items
- 8 to 12 total full news cards when enough processed news exists
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
