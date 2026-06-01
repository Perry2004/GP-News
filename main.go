package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"gpnews/briefing"
	"gpnews/ingest"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type config struct {
	LogLevel               string   `env:"LOG_LEVEL" envDefault:"info"` // debug, info, warn, error
	NewsDataAPIKey         string   `env:"NEWS_DATA_API_KEY"`
	EnableFetching         bool     `env:"ENABLE_FETCHING" envDefault:"true"`
	PersistData            bool     `env:"PERSIST_DATA" envDefault:"true"`
	Model                  string   `env:"MODEL"`
	BaseURL                string   `env:"BASE_URL" envDefault:"https://api.openai.com/v1"`
	LLMAPIKey              string   `env:"LLM_API_KEY"`
	LLMMaxCompletionTokens int64    `env:"LLM_MAX_COMPLETION_TOKENS" envDefault:"0"`
	LLMTemperature         float64  `env:"LLM_TEMPERATURE" envDefault:"0"`
	LLMThinkingLevel       string   `env:"LLM_THINKING_LEVEL" envDefault:"medium"`
	LLMProviderIgnore      []string `env:"LLM_PROVIDER_IGNORE" envSeparator:","`
}

func main() {
	// [TODO] Load config and credentials
	envName := loadEnv()
	cfg, err := env.ParseAs[config]()
	if err != nil {
		panic(err)
	}
	if err := validateConfig(cfg); err != nil {
		panic(err)
	}

	configureLogger(cfg)

	slog.Debug("GP-News configuration loaded", "environment", envName, "config", maskConfigForLogging(cfg))
	slog.Info("Starting GP-News")

	ctx := context.Background()

	marketValues, categoryBuckets, regionBuckets, err := ingest.RetrieveData(ctx, ingest.Config{
		NewsDataAPIKey: cfg.NewsDataAPIKey,
		EnableFetching: cfg.EnableFetching,
		PersistData:    cfg.PersistData,
	})
	if err != nil {
		panic(err)
	}

	llm, err := briefing.NewLLMGenerator(briefing.Config{
		BaseURL:             cfg.BaseURL,
		APIKey:              cfg.LLMAPIKey,
		Model:               cfg.Model,
		MaxCompletionTokens: cfg.LLMMaxCompletionTokens,
		Temperature:         cfg.LLMTemperature,
		ThinkingLevel:       cfg.LLMThinkingLevel,
		ProviderIgnore:      cfg.LLMProviderIgnore,
	})
	if err != nil {
		panic(fmt.Errorf("failed to create LLM generator: %w", err))
	}

	now := time.Now()
	briefingDate := now.Format(time.DateOnly)
	session := briefingSession(now)
	articles := buildArticleInputs(categoryBuckets, regionBuckets)
	marketSnapshot := buildMarketInputs(marketValues)

	briefingEmail, err := llm.GenerateBriefing(ctx, briefing.BriefingAgentInput{
		BriefingDate:   briefingDate,
		Session:        session,
		MarketSnapshot: marketSnapshot,
		Articles:       articles,
	})
	if err != nil {
		panic(fmt.Errorf("failed to generate briefing: %w", err))
	}
	slog.Info("Generated briefing", "output", briefingEmail)

	// [TODO] Render email template

	// [TODO] Send email
}

func loadEnv() string {
	envName := os.Getenv("ENVIRONMENT")
	switch strings.ToLower(strings.TrimSpace(envName)) {
	case "", "dev":
		if err := godotenv.Load(); err != nil {
			panic(fmt.Errorf("failed to load .env file: %w", err))
		}
	case "prod":
		// Skip loading .env
	default:
		panic(fmt.Errorf("invalid environment %q: expected dev or prod", envName))
	}

	return envName
}

func validateConfig(cfg config) error {
	if cfg.EnableFetching && strings.TrimSpace(cfg.NewsDataAPIKey) == "" {
		return fmt.Errorf("invalid config: NEWS_DATA_API_KEY is required when ENABLE_FETCHING=true")
	}
	return nil
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
			ID:        value.ID,
			Name:      value.Name,
			Category:  value.Category,
			Symbol:    value.Symbol,
			Level:     fmt.Sprintf("%g", value.Value),
			Timestamp: value.Timestamp.Format(time.RFC3339),
			Source:    value.Source,
		})
	}
	return marketInputs
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

func configureLogger(cfg config) {
	var logLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(cfg.LogLevel)) {
	case "debug":
		logLevel = slog.LevelDebug
	case "", "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		panic(fmt.Errorf("invalid log level %q: expected debug, info, warn, or error", cfg.LogLevel))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: maskSensitiveLogAttr,
	}))
	slog.SetDefault(logger)
}

func maskSensitiveLogAttr(_ []string, attr slog.Attr) slog.Attr {
	maskedValue, changed := maskLogValue(attr.Value.Any())
	if changed {
		attr.Value = slog.AnyValue(maskedValue)
	}
	return attr
}

func maskLogValue(value any) (any, bool) {
	switch value := value.(type) {
	case string:
		masked := maskSensitiveURLSubstrings(value)
		return masked, masked != value
	case error:
		masked := maskSensitiveURLSubstrings(value.Error())
		return masked, masked != value.Error()
	case fmt.Stringer:
		stringValue := value.String()
		masked := maskSensitiveURLSubstrings(stringValue)
		return masked, masked != stringValue
	default:
		return value, false
	}
}

var httpURLPattern = regexp.MustCompile(`https?://[^\s"'<>()]+`)

func maskSensitiveURLSubstrings(value string) string {
	return httpURLPattern.ReplaceAllStringFunc(value, maskSensitiveURLQuery)
}

func maskSensitiveURLQuery(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsedURL.Query()
	masked := false
	const mask = "********"
	for key, values := range query {
		if !isSensitiveQueryKey(key) {
			continue
		}

		for i := range values {
			values[i] = mask
		}
		query[key] = values
		masked = true
	}

	if !masked {
		return rawURL
	}

	parsedURL.RawQuery = strings.ReplaceAll(query.Encode(), url.QueryEscape(mask), mask)
	return parsedURL.String()
}

func isSensitiveQueryKey(key string) bool {
	switch strings.ToLower(key) {
	case "apikey", "api_key":
		return true
	default:
		return false
	}
}

func maskConfigForLogging(cfg config) config {
	masked := cfg
	cfgValue := reflect.ValueOf(&masked).Elem()
	cfgType := cfgValue.Type()

	for i := range cfgValue.NumField() {
		field := cfgValue.Field(i)
		fieldName := cfgType.Field(i).Name

		if field.Kind() == reflect.String && strings.Contains(strings.ToLower(fieldName), "key") {
			field.SetString("********")
		}
	}

	return masked
}
