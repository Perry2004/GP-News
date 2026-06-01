package briefing

type ArticleInput struct {
	ID                 string `json:"id"`
	BucketID           string `json:"bucket_id"`
	BucketName         string `json:"bucket_name"`
	Title              string `json:"title"`
	Link               string `json:"link"`
	ExtractedTitle     string `json:"extracted_title"`
	ExtractedContent   string `json:"extracted_content"`
	ExtractedWordCount int    `json:"extracted_word_count"`
	ExtractionError    string `json:"extraction_error"`
}

type ProcessNewsInput struct {
	BriefingDate string       `json:"briefing_date"`
	Article      ArticleInput `json:"article"`
}

type ProcessedNews struct {
	ArticleID            string   `json:"article_id" jsonschema_description:"Input article ID."`
	Headline             string   `json:"headline" jsonschema_description:"Clean headline."`
	Summary              string   `json:"summary" jsonschema_description:"One to two sentence factual summary."`
	Entities             []string `json:"entities" jsonschema_description:"Companies, people, institutions, countries, or assets."`
	Region               string   `json:"region" jsonschema_description:"Primary region or Global."`
	AssetClasses         []string `json:"asset_classes" jsonschema_description:"Relevant asset classes, such as FX, rates, equities, commodities, crypto, policy, or technology."`
	MarketRelevanceScore float64  `json:"market_relevance_score" jsonschema:"minimum=0,maximum=10" jsonschema_description:"0 to 10 market relevance score."`
	NoveltyScore         float64  `json:"novelty_score" jsonschema:"minimum=0,maximum=10" jsonschema_description:"0 to 10 novelty score."`
	WhyItMatters         string   `json:"why_it_matters" jsonschema_description:"One short market, policy, geopolitical, or technology relevance sentence."`
	PossibleMarketImpact string   `json:"possible_market_impact" jsonschema_description:"Concise impact read-through, or 'No clear market impact.'"`
	KeepForBriefing      bool     `json:"keep_for_briefing" jsonschema_description:"Whether this should be eligible for the final briefing."`
	Confidence           string   `json:"confidence" jsonschema:"enum=High,enum=Medium,enum=Low"`
	SourceURL            string   `json:"source_url" jsonschema_description:"Article URL."`
	SourceName           string   `json:"source_name" jsonschema_description:"Source name inferred from the URL or article metadata."`
}

func processedNewsSchema() any {
	return schemaFor[ProcessedNews]()
}
