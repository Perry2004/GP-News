package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"

	"gpnews/data"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type config struct {
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"` // debug, info, warn, error
	NewsDataAPIKey string `env:"NEWS_DATA_API_KEY"`
	EnableFetching bool   `env:"ENABLE_FETCHING" envDefault:"true"`
	PersistData    bool   `env:"PERSIST_DATA" envDefault:"true"`
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
	marketValues, categoryBuckets, regionBuckets, err := data.RetrieveData(ctx, data.Config{
		NewsDataAPIKey: cfg.NewsDataAPIKey,
		EnableFetching: cfg.EnableFetching,
		PersistData:    cfg.PersistData,
	})
	if err != nil {
		panic(err)
	}
	_, _, _ = marketValues, categoryBuckets, regionBuckets

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
