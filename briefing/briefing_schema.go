package briefing

import "github.com/invopop/jsonschema"

type MarketInput struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Category    string               `json:"category"`
	Symbol      string               `json:"symbol"`
	Level       string               `json:"level"`
	DailyChange string               `json:"daily_change"`
	Timestamp   string               `json:"timestamp"`
	History     []MarketHistoryPoint `json:"history"`
	Source      string               `json:"source"`
}

type MarketHistoryPoint struct {
	Timestamp string `json:"timestamp"`
	Close     string `json:"close"`
}

type BriefingAgentInput struct {
	BriefingDate   string         `json:"briefing_date"`
	Session        string         `json:"session"`
	MarketSnapshot []MarketInput  `json:"market_snapshot"`
	Articles       []ArticleInput `json:"articles"`
}

type ThemeCluster struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Topic            string   `json:"topic"`
	Region           string   `json:"region"`
	AssetClasses     []string `json:"asset_classes"`
	ImportanceScore  float64  `json:"importance_score"`
	ProcessedNewsIDs []string `json:"processed_news_ids"`
}

type ReviewedNews struct {
	News              ProcessedNews `json:"news"`
	PriorityScore     float64       `json:"priority_score" jsonschema:"minimum=0,maximum=10"`
	ReviewNote        string        `json:"review_note"`
	Corrections       []string      `json:"corrections"`
	AdditionalContext []string      `json:"additional_context"`
}

type ReviewSummary struct {
	SelectionRationale string `json:"selection_rationale"`
	GlobalContext      string `json:"global_context"`
}

type BriefingInput struct {
	BriefingDate   string         `json:"briefing_date"`
	Session        string         `json:"session"`
	MarketSnapshot []MarketInput  `json:"market_snapshot"`
	ReviewedNews   []ReviewedNews `json:"reviewed_news"`
	ReviewSummary  ReviewSummary  `json:"review_summary"`
	ThemeClusters  []ThemeCluster `json:"theme_clusters"`
}

type GenerationResult struct {
	Email      BriefingEmail `json:"email"`
	FinalInput BriefingInput `json:"final_input"`
}

type BriefingEmail struct {
	Subject                string          `json:"subject" jsonschema:"minLength=12,maxLength=120" jsonschema_description:"Generated email subject line for this specific briefing. Must be concise and mention the main market driver or criticality, not a generic desk name."`
	CriticalityScore       float64         `json:"criticality_score" jsonschema:"minimum=0,maximum=10"`
	PriorityLevel          string          `json:"priority_level" jsonschema:"enum=Low,enum=Watch,enum=Important,enum=Critical"`
	HighPriorityTag        bool            `json:"high_priority_tag"`
	MainDriver             string          `json:"main_driver"`
	TodaysSignal           string          `json:"todays_signal"`
	ReadThisFirst          []string        `json:"read_this_first"`
	MarketSnapshot         MarketSnapshot  `json:"market_snapshot"`
	MacroDataWatch         []string        `json:"macro_data_watch"`
	PolicySignalWatch      []string        `json:"policy_signal_watch"`
	TopNewsByTopic         TopNewsByTopic  `json:"top_news_by_topic"`
	RegionalRadar          []RegionalRadar `json:"regional_radar"`
	ToneFramingDifferences []string        `json:"tone_framing_differences"`
	TechTendency           []string        `json:"tech_tendency"`
	PolymarketWatch        []string        `json:"polymarket_watch"`
	WatchNext              []string        `json:"watch_next"`
	WhyThisMattersToday    string          `json:"why_this_matters_today"`
}

type BriefingEmailDraft struct {
	Subject                string          `json:"subject" jsonschema:"minLength=12,maxLength=120" jsonschema_description:"Generated email subject line for this specific briefing. Must be concise and mention the main market driver or criticality, not a generic desk name."`
	CriticalityScore       float64         `json:"criticality_score" jsonschema:"minimum=0,maximum=10"`
	PriorityLevel          string          `json:"priority_level" jsonschema:"enum=Low,enum=Watch,enum=Important,enum=Critical"`
	HighPriorityTag        bool            `json:"high_priority_tag"`
	MainDriver             string          `json:"main_driver"`
	TodaysSignal           string          `json:"todays_signal"`
	ReadThisFirst          []string        `json:"read_this_first"`
	MarketDrivers          []MarketDriver  `json:"market_drivers" jsonschema_description:"One concise driver for each supplied market_snapshot item, keyed by the supplied market id. Do not include market_snapshot in this output."`
	MacroDataWatch         []string        `json:"macro_data_watch"`
	PolicySignalWatch      []string        `json:"policy_signal_watch"`
	TopNewsByTopic         TopNewsByTopic  `json:"top_news_by_topic"`
	RegionalRadar          []RegionalRadar `json:"regional_radar"`
	ToneFramingDifferences []string        `json:"tone_framing_differences"`
	TechTendency           []string        `json:"tech_tendency"`
	PolymarketWatch        []string        `json:"polymarket_watch"`
	WatchNext              []string        `json:"watch_next"`
	WhyThisMattersToday    string          `json:"why_this_matters_today"`
}

type MarketDriver struct {
	ID     string `json:"id" jsonschema_description:"Market id copied exactly from the supplied market_snapshot input."`
	Driver string `json:"driver" jsonschema_description:"Concise English market driver for this market item."`
}

type MarketSnapshot struct {
	EquityIndices         []MarketSnapshotItem `json:"equity_indices"`
	FX                    []MarketSnapshotItem `json:"fx"`
	RatesBonds            []MarketSnapshotItem `json:"rates_bonds"`
	CommoditiesCryptoRisk []MarketSnapshotItem `json:"commodities_crypto_risk"`
}

type MarketSnapshotItem struct {
	Asset       string `json:"asset"`
	Level       string `json:"level"`
	DailyChange string `json:"daily_change" jsonschema:"pattern=^$|^[+-][0-9]+([.][0-9]{1\\,2})? [(][+-][0-9]+([.][0-9]{1\\,2})?%[)]$" jsonschema_description:"Empty only when no daily comparison was supplied. Otherwise copy the supplied daily_change exactly in '+absolute (+percent%)' format, for example '+19.90 (+0.26%)' or '-120.40 (-1.35%)'. Do not output percent-only values."`
	Timestamp   string `json:"timestamp"`
	Driver      string `json:"driver"`
	Source      string `json:"source"`
}

type TopNewsByTopic struct {
	MarketsMacro        []NewsCard `json:"markets_macro" jsonschema_description:"Markets and macro full news cards. The total number of cards across all top_news_by_topic arrays must be 5 to 15."`
	PoliticsPolicy      []NewsCard `json:"politics_policy" jsonschema_description:"Politics and policy full news cards. The total number of cards across all top_news_by_topic arrays must be 5 to 15."`
	WarGeopoliticalRisk []NewsCard `json:"war_geopolitical_risk" jsonschema_description:"War and geopolitical risk full news cards. The total number of cards across all top_news_by_topic arrays must be 5 to 15."`
	TechnologyAI        []NewsCard `json:"technology_ai" jsonschema_description:"Technology and AI full news cards. The total number of cards across all top_news_by_topic arrays must be 5 to 15."`
}

type NewsCard struct {
	Topic         string           `json:"topic" jsonschema:"enum=Markets & Macro,enum=Politics & Policy,enum=War & Geopolitical Risk,enum=Technology & AI"`
	Region        string           `json:"region"`
	Headline      string           `json:"headline"`
	Summary       string           `json:"summary"`
	WhyItMatters  string           `json:"why_it_matters"`
	Sources       []BriefingSource `json:"sources" jsonschema_description:"Source labels and URLs for this news card, taken from supplied reviewed news."`
	PriorityScore float64          `json:"priority_score" jsonschema:"minimum=0,maximum=10"`
	Confidence    string           `json:"confidence" jsonschema:"enum=High,enum=Medium,enum=Low"`
	MustRead      bool             `json:"must_read"`
}

type RegionalRadar struct {
	Region   string           `json:"region"`
	Sentence string           `json:"sentence"`
	Sources  []BriefingSource `json:"sources" jsonschema_description:"Source labels and URLs for this regional radar item, taken from supplied reviewed news."`
}

type BriefingSource struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func briefingEmailSchema() any {
	return schemaFor[BriefingEmail]()
}

func briefingEmailDraftSchema() any {
	return schemaFor[BriefingEmailDraft]()
}

func schemaFor[T any]() any {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}

	var output T
	return reflector.Reflect(output)
}
