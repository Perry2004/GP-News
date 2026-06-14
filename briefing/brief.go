package briefing

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	finalNewsCardMin                   = 5
	finalNewsCardMax                   = 15
	finalBriefingChatCompletionTimeout = 6 * time.Minute
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
	return g.generateFinalBriefingFromInput(ctx, input)
}

func (g *LLMGenerator) GenerateFinalBriefing(ctx context.Context, input BriefingInput) (BriefingEmail, error) {
	return g.generateFinalBriefingFromInput(ctx, input)
}

func (g *LLMGenerator) generateFinalBriefingFromInput(ctx context.Context, input BriefingInput) (BriefingEmail, error) {
	slog.Debug("Generating final briefing",
		"briefing_date", input.BriefingDate,
		"session", input.Session,
		"market_item_count", len(input.MarketSnapshot),
		"reviewed_news_count", len(input.ReviewedNews),
	)
	g.persistCacheJSON(finalBriefingInputCacheFileName, input)

	output, err := g.generateFinalBriefingWithPrompt(ctx, input, briefingSystemPrompt())
	if err != nil {
		return BriefingEmail{}, err
	}
	if err := validateFinalNewsCardCount(output); err == nil {
		g.persistCacheJSON(finalBriefingOutputCacheFileName, output)
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
	g.persistCacheJSON(finalBriefingOutputCacheFileName, output)
	return output, nil
}

func (g *LLMGenerator) generateFinalBriefingWithPrompt(ctx context.Context, input BriefingInput, systemPrompt string) (BriefingEmail, error) {
	var draft BriefingEmailDraft
	err := generateStructured(ctx, g, structuredRequest[BriefingEmailDraft]{
		Name:        "briefing_email",
		Description: "Email-ready GP News briefing draft JSON with market drivers keyed by supplied market ids.",
		Schema:      briefingEmailDraftSchema(),
		System:      systemPrompt,
		Input:       input,
		Output:      &draft,
		Timeout:     finalBriefingChatCompletionTimeout,
	})
	if err != nil {
		return BriefingEmail{}, err
	}
	g.persistCacheJSON(finalBriefingDraftCacheFileName, draft)
	return mergeBriefingDraft(input, draft), nil
}

func mergeBriefingDraft(input BriefingInput, draft BriefingEmailDraft) BriefingEmail {
	return BriefingEmail{
		Subject:                draft.Subject,
		CriticalityScore:       draft.CriticalityScore,
		PriorityLevel:          draft.PriorityLevel,
		HighPriorityTag:        draft.HighPriorityTag,
		MainDriver:             draft.MainDriver,
		TodaysSignal:           draft.TodaysSignal,
		ReadThisFirst:          draft.ReadThisFirst,
		MarketSnapshot:         buildDeterministicMarketSnapshot(input.MarketSnapshot, draft.MarketDrivers),
		MacroDataWatch:         draft.MacroDataWatch,
		PolicySignalWatch:      draft.PolicySignalWatch,
		TopNewsByTopic:         draft.TopNewsByTopic,
		RegionalRadar:          draft.RegionalRadar,
		ToneFramingDifferences: draft.ToneFramingDifferences,
		TechTendency:           draft.TechTendency,
		PolymarketWatch:        draft.PolymarketWatch,
		WatchNext:              draft.WatchNext,
		WhyThisMattersToday:    draft.WhyThisMattersToday,
	}
}

func buildDeterministicMarketSnapshot(markets []MarketInput, drivers []MarketDriver) MarketSnapshot {
	knownIDs := make(map[string]bool, len(markets))
	for _, market := range markets {
		knownIDs[market.ID] = true
	}

	driverByID := make(map[string]string, len(drivers))
	for _, driver := range drivers {
		id := strings.TrimSpace(driver.ID)
		if id == "" {
			continue
		}
		if !knownIDs[id] {
			slog.Warn("Ignoring market driver for unknown market id", "market_id", id)
			continue
		}
		driverText := strings.TrimSpace(driver.Driver)
		if driverText == "" {
			continue
		}
		driverByID[id] = driverText
	}

	var snapshot MarketSnapshot
	for _, market := range markets {
		item := MarketSnapshotItem{
			Asset:       market.Name,
			Level:       market.Level,
			DailyChange: market.DailyChange,
			Timestamp:   market.Timestamp,
			Driver:      driverByID[market.ID],
			Source:      market.Source,
		}
		if strings.TrimSpace(item.Driver) == "" {
			item.Driver = "No specific driver provided."
		}

		switch market.Category {
		case "equity_index":
			snapshot.EquityIndices = append(snapshot.EquityIndices, item)
		case "fx":
			snapshot.FX = append(snapshot.FX, item)
		case "rates":
			snapshot.RatesBonds = append(snapshot.RatesBonds, item)
		case "commodity", "crypto", "risk":
			snapshot.CommoditiesCryptoRisk = append(snapshot.CommoditiesCryptoRisk, item)
		default:
			slog.Warn("Ignoring market input with unknown category",
				"market_id", market.ID,
				"category", market.Category,
			)
		}
	}
	return snapshot
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
- generate subject as a specific email subject line for this briefing, mentioning the main market driver or criticality; never use a generic desk name
- exactly 3 read_this_first items
- 5 to 15 total full news cards across top_news_by_topic when enough reviewed news exists
- total full news cards means len(markets_macro) + len(politics_policy) + len(war_geopolitical_risk) + len(technology_ai)
- never exceed 15 total full news cards
- any top_news_by_topic category may be empty when it has no strong reviewed news
- 5 to 8 regional_radar items when enough processed news exists
- 2 to 3 watch_next items
- do not output market_snapshot; it is assembled deterministically after this draft
- output market_drivers only, one concise driver for each supplied market_snapshot item, using each supplied market id exactly
- use supplied market levels, daily_change, timestamps, and recent 5-day daily close history only as context for market_drivers
- never invent or copy deterministic market fields into market_drivers; write only the id and driver text
- never claim a daily move in a driver unless daily_change is present in supplied market data
- source every major news card from supplied reviewed news only
- every top_news_by_topic card must include sources as label/url objects from supplied reviewed news
- every regional_radar item must include sources as label/url objects from supplied reviewed news
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
