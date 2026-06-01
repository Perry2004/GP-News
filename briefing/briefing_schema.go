package briefing

import "github.com/invopop/jsonschema"

type MarketInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Symbol      string `json:"symbol"`
	Level       string `json:"level"`
	DailyChange string `json:"daily_change"`
	Timestamp   string `json:"timestamp"`
	Source      string `json:"source"`
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

type BriefingEmail struct {
	Subject                string           `json:"subject"`
	CriticalityScore       float64          `json:"criticality_score" jsonschema:"minimum=0,maximum=10"`
	PriorityLevel          string           `json:"priority_level" jsonschema:"enum=Low,enum=Watch,enum=Important,enum=Critical"`
	HighPriorityTag        bool             `json:"high_priority_tag"`
	MainDriver             string           `json:"main_driver"`
	TodaysSignal           string           `json:"todays_signal"`
	ReadThisFirst          []string         `json:"read_this_first"`
	MarketSnapshot         MarketSnapshot   `json:"market_snapshot"`
	MacroDataWatch         []string         `json:"macro_data_watch"`
	PolicySignalWatch      []string         `json:"policy_signal_watch"`
	TopNewsByTopic         TopNewsByTopic   `json:"top_news_by_topic"`
	RegionalRadar          []RegionalRadar  `json:"regional_radar"`
	ToneFramingDifferences []string         `json:"tone_framing_differences"`
	TechTendency           []string         `json:"tech_tendency"`
	PolymarketWatch        []string         `json:"polymarket_watch"`
	WatchNext              []string         `json:"watch_next"`
	WhyThisMattersToday    string           `json:"why_this_matters_today"`
	Sources                []BriefingSource `json:"sources"`
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
	DailyChange string `json:"daily_change"`
	Timestamp   string `json:"timestamp"`
	Driver      string `json:"driver"`
	Source      string `json:"source"`
}

type TopNewsByTopic struct {
	MarketsMacro        []NewsCard `json:"markets_macro"`
	PoliticsPolicy      []NewsCard `json:"politics_policy"`
	WarGeopoliticalRisk []NewsCard `json:"war_geopolitical_risk"`
	TechnologyAI        []NewsCard `json:"technology_ai"`
}

type NewsCard struct {
	Topic         string   `json:"topic" jsonschema:"enum=Markets & Macro,enum=Politics & Policy,enum=War & Geopolitical Risk,enum=Technology & AI"`
	Region        string   `json:"region"`
	Headline      string   `json:"headline"`
	Summary       string   `json:"summary"`
	WhyItMatters  string   `json:"why_it_matters"`
	Sources       []string `json:"sources"`
	PriorityScore float64  `json:"priority_score" jsonschema:"minimum=0,maximum=10"`
	Confidence    string   `json:"confidence" jsonschema:"enum=High,enum=Medium,enum=Low"`
	MustRead      bool     `json:"must_read"`
}

type RegionalRadar struct {
	Region   string `json:"region"`
	Sentence string `json:"sentence"`
}

type BriefingSource struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func briefingEmailSchema() any {
	return schemaFor[BriefingEmail]()
}

func schemaFor[T any]() any {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}

	var output T
	return reflector.Reflect(output)
}
