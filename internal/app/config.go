package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type config struct {
	LogLevel                    string   `env:"LOG_LEVEL" envDefault:"info"` // debug, info, warn, error
	NewsDataAPIKey              string   `env:"NEWS_DATA_API_KEY"`
	FreshFrom                   string   `env:"FRESH_FROM" envDefault:"fetching"` // fetching, summarization, review, briefing, cached
	PersistData                 bool     `env:"PERSIST_DATA" envDefault:"false"`  // whether to store to cache json for debugging and auditing.
	Model                       string   `env:"MODEL"`                            // LLM model name
	BaseURL                     string   `env:"BASE_URL" envDefault:"https://api.openai.com/v1"`
	LLMAPIKey                   string   `env:"LLM_API_KEY"`
	LLMMaxCompletionTokens      int64    `env:"LLM_MAX_COMPLETION_TOKENS" envDefault:"0"`
	LLMTemperature              float64  `env:"LLM_TEMPERATURE" envDefault:"0"`
	LLMThinkingLevel            string   `env:"LLM_THINKING_LEVEL" envDefault:"medium"`
	LLMProviderIgnore           []string `env:"LLM_PROVIDER_IGNORE" envSeparator:","` // ignore open router providers
	EnableEmailSending          bool     `env:"ENABLE_EMAIL_SENDING" envDefault:"true"`
	EmailFrom                   string   `env:"EMAIL_FROM"` // from email address
	EmailTo                     []string `env:"EMAIL_TO" envSeparator:","`
	AWSSESRegion                string   `env:"AWS_SES_REGION"`
	CacheDir                    string   `env:"CACHE_DIR" envDefault:"cache"`
	BriefingHistoryTable        string   `env:"BRIEFING_HISTORY_TABLE"` // DynamoDB table name for briefing history. If empty, briefing history won't be stored.
	BriefingHistoryLookbackDays int      `env:"BRIEFING_HISTORY_LOOKBACK_DAYS" envDefault:"7"`
	BriefingHistoryTTLDays      int      `env:"BRIEFING_HISTORY_TTL_DAYS" envDefault:"14"`
}

// Indicates from which step it should generate/retrieve fresh data.
type FreshFrom string

const (
	FreshFromFetching      FreshFrom = "fetching"      // fetch fresh news data.
	FreshFromSummarization FreshFrom = "summarization" // generate per-news summaries.
	FreshFromReview        FreshFrom = "review"        // review processed news.
	FreshFromBriefing      FreshFrom = "briefing"      // generate briefing contents.
	FreshFromCached        FreshFrom = "cached"        // use all as cached data.
)

// Load .env if ENVIRONMENT is dev.Retruns the environment name.
func loadEnv() (string, error) {
	envName := os.Getenv("ENVIRONMENT")
	switch strings.ToLower(strings.TrimSpace(envName)) {
	case "", "dev": // Default to dev env
		if err := godotenv.Load(); err != nil {
			return "", fmt.Errorf("failed to load .env file: %w", err)
		}
	case "prod":
		// Skip loading .env
	default:
		return "", fmt.Errorf("invalid environment %q: expected dev or prod", envName)
	}

	return envName, nil
}

func validateConfig(cfg config) error {
	freshFrom, err := freshFromConfig(cfg)
	if err != nil {
		return err
	}
	if freshFrom == FreshFromFetching && strings.TrimSpace(cfg.NewsDataAPIKey) == "" {
		return fmt.Errorf("invalid config: NEWS_DATA_API_KEY is required when FRESH_FROM=fetching")
	}
	if freshFrom != FreshFromCached {
		if strings.TrimSpace(cfg.BaseURL) == "" {
			return fmt.Errorf("invalid config: BASE_URL is required when FRESH_FROM is not cached")
		}
		if strings.TrimSpace(cfg.LLMAPIKey) == "" {
			return fmt.Errorf("invalid config: LLM_API_KEY is required when FRESH_FROM is not cached")
		}
		if strings.TrimSpace(cfg.Model) == "" {
			return fmt.Errorf("invalid config: MODEL is required when FRESH_FROM is not cached")
		}
	}
	if cfg.EnableEmailSending {
		if strings.TrimSpace(cfg.EmailFrom) == "" {
			return fmt.Errorf("invalid config: EMAIL_FROM is required when ENABLE_EMAIL_SENDING=true")
		}
		if len(nonEmptyStrings(cfg.EmailTo)) == 0 {
			return fmt.Errorf("invalid config: EMAIL_TO is required when ENABLE_EMAIL_SENDING=true")
		}
	}
	if strings.TrimSpace(cfg.BriefingHistoryTable) != "" {
		if cfg.BriefingHistoryLookbackDays <= 0 {
			return fmt.Errorf("invalid config: BRIEFING_HISTORY_LOOKBACK_DAYS must be positive when BRIEFING_HISTORY_TABLE is set")
		}
		if cfg.BriefingHistoryTTLDays <= 0 {
			return fmt.Errorf("invalid config: BRIEFING_HISTORY_TTL_DAYS must be positive when BRIEFING_HISTORY_TABLE is set")
		}
	}
	return nil
}

func freshFromConfig(cfg config) (FreshFrom, error) {
	value := strings.ToLower(strings.TrimSpace(cfg.FreshFrom))
	freshFrom := FreshFrom(value)
	switch freshFrom {
	case FreshFromFetching, FreshFromSummarization, FreshFromReview, FreshFromBriefing, FreshFromCached:
		return freshFrom, nil
	default:
		return "", fmt.Errorf("invalid FRESH_FROM %q: expected fetching, summarization, review, briefing, or cached", cfg.FreshFrom)
	}
}

func freshFromNeedsRawData(freshFrom FreshFrom) bool {
	switch freshFrom {
	case FreshFromFetching, FreshFromSummarization, FreshFromReview:
		return true
	default:
		return false
	}
}

func nonEmptyStrings(values []string) []string {
	nonEmpty := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			nonEmpty = append(nonEmpty, value)
		}
	}
	return nonEmpty
}
