package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type config struct {
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"` // debug, info, warn, error
	NewsDataAPIKey string `env:"NEWS_DATA_API_KEY"`
	EnableFetching bool   `env:"ENABLE_FETCHING" envDefault:"true"`
	PersistData    bool   `env:"PERSIST_DATA" envDefault:"true"`
}

const (
	marketValuesFilePath = "data/market_values.json"
	newsDataFilePath     = "data/news_data.json"
)

type cachedNewsDataJSON struct {
	CategoryBuckets []NewsArticleBucket `json:"category_buckets"`
	RegionBuckets   []NewsArticleBucket `json:"region_buckets"`
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

	var marketValues []MarketValue
	var categoryBuckets []NewsArticleBucket
	var regionBuckets []NewsArticleBucket

	ctx := context.Background()
	if cfg.EnableFetching {
		// Fetch market data
		marketValuesChan := make(chan struct {
			Values   []MarketValue
			Failures []FetchFailure
		}, 1)

		categoryBucketsChan := make(chan struct {
			Buckets  []NewsArticleBucket
			Failures []NewsDataFetchFailure
		}, 1)

		regionBucketsChan := make(chan struct {
			Buckets  []NewsArticleBucket
			Failures []NewsDataFetchFailure
		}, 1)

		go func() {
			marketValues, fetchFailures := FetchYahooMarketValues(ctx, yahooFinanceInstruments())
			marketValuesChan <- struct {
				Values   []MarketValue
				Failures []FetchFailure
			}{Values: marketValues, Failures: fetchFailures}
		}()
		go func() {
			// News data shall be fetched sequentially to avoid getting rate limited
			categoryBuckets, categoryFetchFailures := FetchNewsDataCategoryArticles(ctx, cfg.NewsDataAPIKey)
			categoryBucketsChan <- struct {
				Buckets  []NewsArticleBucket
				Failures []NewsDataFetchFailure
			}{Buckets: categoryBuckets, Failures: categoryFetchFailures}

			regionBuckets, regionFetchFailures := FetchNewsDataRegionArticles(ctx, cfg.NewsDataAPIKey)
			regionBucketsChan <- struct {
				Buckets  []NewsArticleBucket
				Failures []NewsDataFetchFailure
			}{Buckets: regionBuckets, Failures: regionFetchFailures}
		}()

		for i := 0; i < 3; i++ {
			select {
			case marketData := <-marketValuesChan:
				marketValues = marketData.Values
				fetchFailures := marketData.Failures
				slog.Info("Yahoo Finance market snapshot fetched", "success_count", len(marketValues), "failure_count", len(fetchFailures))
			case categoryBucketsValues := <-categoryBucketsChan:
				categoryBuckets = categoryBucketsValues.Buckets
				categoryFetchFailures := categoryBucketsValues.Failures
				slog.Info("NewsData category articles fetched", "bucket_count", len(categoryBuckets), "article_count", countNewsArticles(categoryBuckets), "failure_count", len(categoryFetchFailures))
			case regionBucketsValues := <-regionBucketsChan:
				regionBuckets = regionBucketsValues.Buckets
				regionFetchFailures := regionBucketsValues.Failures
				slog.Info("NewsData region articles fetched", "bucket_count", len(regionBuckets), "article_count", countNewsArticles(regionBuckets), "failure_count", len(regionFetchFailures))
			}
		}

		if cfg.PersistData {
			if err := StoreDataJSON(marketValuesFilePath, newsDataFilePath, marketValues, categoryBuckets, regionBuckets); err != nil {
				slog.Error("Failed to store data JSON", "error", err)
			} else {
				slog.Info("Data JSON files stored successfully", "market_file", marketValuesFilePath, "news_file", newsDataFilePath)
			}
		} else {
			slog.Info("Data persistence disabled; skipping JSON file storage")
		}
	} else {
		slog.Info("Fetching disabled; loading cached data JSON", "market_file", marketValuesFilePath, "news_file", newsDataFilePath)
		marketValues, categoryBuckets, regionBuckets, err = LoadDataJSON(marketValuesFilePath, newsDataFilePath)
		if err != nil {
			panic(err)
		}
		slog.Info("Cached data JSON loaded",
			"market_count", len(marketValues),
			"category_bucket_count", len(categoryBuckets),
			"region_bucket_count", len(regionBuckets),
			"article_count", countNewsArticles(categoryBuckets)+countNewsArticles(regionBuckets),
		)
	}

	// [TODO] Process and dedupe article JSONs

	// [TODO] Fetch article URLs

	// [TODO] Extract article content

	// [TODO] Invoke LLM to generate summaries

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
		Level: logLevel,
	}))
	slog.SetDefault(logger)
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

func StoreDataJSON(marketFilePath string, newsFilePath string, marketValues []MarketValue, categoryBuckets []NewsArticleBucket, regionBuckets []NewsArticleBucket) error {
	if err := writeJSONToFile(marketFilePath, marketValues); err != nil {
		return fmt.Errorf("failed to write market values JSON: %w", err)
	}
	if err := writeJSONToFile(newsFilePath, cachedNewsDataJSON{
		CategoryBuckets: categoryBuckets,
		RegionBuckets:   regionBuckets,
	}); err != nil {
		return fmt.Errorf("failed to write news data JSON: %w", err)
	}
	return nil
}

func LoadDataJSON(marketFilePath string, newsFilePath string) ([]MarketValue, []NewsArticleBucket, []NewsArticleBucket, error) {
	var marketValues []MarketValue
	if err := readJSONFromFile(marketFilePath, &marketValues); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read market values JSON: %w", err)
	}

	var newsData cachedNewsDataJSON
	if err := readJSONFromFile(newsFilePath, &newsData); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read news data JSON: %w", err)
	}

	return marketValues, newsData.CategoryBuckets, newsData.RegionBuckets, nil
}

func writeJSONToFile(filePath string, data any) error {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func readJSONFromFile(filePath string, target any) error {
	jsonBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("unmarshal JSON: %w", err)
	}
	return nil
}
