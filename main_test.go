package main

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"gpnews/ingest"

	"github.com/caarlos0/env/v11"
)

func TestMaskConfigForLogging(t *testing.T) {
	cfg := config{
		LogLevel:       "debug",
		NewsDataAPIKey: "secret-news-data-key",
	}

	masked := maskConfigForLogging(cfg)

	if masked.LogLevel != cfg.LogLevel {
		t.Fatalf("LogLevel = %q, want %q", masked.LogLevel, cfg.LogLevel)
	}
	if masked.NewsDataAPIKey == cfg.NewsDataAPIKey {
		t.Fatal("NewsDataAPIKey was not masked")
	}
	if cfg.NewsDataAPIKey != "secret-news-data-key" {
		t.Fatal("maskConfigForLogging mutated the original config")
	}
}

func TestConfigParsesProviderIgnoreList(t *testing.T) {
	t.Setenv("LLM_PROVIDER_IGNORE", "akashml,morph")

	cfg, err := env.ParseAs[config]()
	if err != nil {
		t.Fatalf("ParseAs() error = %v", err)
	}

	if len(cfg.LLMProviderIgnore) != 2 || cfg.LLMProviderIgnore[0] != "akashml" || cfg.LLMProviderIgnore[1] != "morph" {
		t.Fatalf("LLMProviderIgnore = %#v, want [akashml morph]", cfg.LLMProviderIgnore)
	}
}

func TestBuildMarketInputsIncludesDailyChangeAndHistory(t *testing.T) {
	marketTime := time.Unix(1717000000, 0)
	historyTime := time.Unix(1716739200, 0)
	marketInputs := buildMarketInputs([]ingest.MarketValue{
		{
			ID:                 "sp500",
			Name:               "S&P 500",
			Category:           "equity_index",
			Symbol:             "^GSPC",
			Value:              5250.75,
			DailyChange:        25.5,
			DailyChangePercent: 0.48801492847508446,
			DailyChangeValid:   true,
			Timestamp:          marketTime,
			History: []ingest.MarketHistoryPoint{
				{Timestamp: historyTime, Close: 5225.25},
				{Timestamp: marketTime, Close: 5250.75},
			},
			Source: "yahoo_chart_api",
		},
	})

	if len(marketInputs) != 1 {
		t.Fatalf("market input count = %d, want 1", len(marketInputs))
	}
	input := marketInputs[0]
	if input.DailyChange != "+25.50 (+0.49%)" {
		t.Fatalf("DailyChange = %q, want %q", input.DailyChange, "+25.50 (+0.49%)")
	}
	if len(input.History) != 2 {
		t.Fatalf("History length = %d, want 2", len(input.History))
	}
	if input.History[0].Timestamp != historyTime.Format(time.RFC3339) {
		t.Fatalf("History[0].Timestamp = %q, want %q", input.History[0].Timestamp, historyTime.Format(time.RFC3339))
	}
	if input.History[0].Close != "5225.25" {
		t.Fatalf("History[0].Close = %q, want 5225.25", input.History[0].Close)
	}
}

func TestMaskSensitiveLogAttrMasksAPIKeyInDirectErrorLogs(t *testing.T) {
	var log bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&log, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: maskSensitiveLogAttr,
	}))

	logger.Warn(
		"NewsData fetch failed",
		"bucket", "middle_east",
		"endpoint", "latest",
		"error", errors.New(`request failed for latest: Get "https://newsdata.io/api/1/latest?apikey=secret-api-key&country=lb%2Ceg&size=10": GET https://newsdata.io/api/1/latest?apikey=secret-api-key&country=lb%2Ceg&size=10 giving up after 1 attempt(s): context deadline exceeded`),
	)

	output := log.String()
	if strings.Contains(output, "secret-api-key") {
		t.Fatalf("log output leaked API key: %s", output)
	}
	if strings.Count(output, "apikey=********") != 2 {
		t.Fatalf("log output did not mask both API key URLs: %s", output)
	}
	if !strings.Contains(output, "country=lb%2Ceg") {
		t.Fatalf("log output lost non-sensitive query values: %s", output)
	}
}

func TestMaskSensitiveLogAttrMasksAPIKeyInURL(t *testing.T) {
	var log bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&log, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: maskSensitiveLogAttr,
	}))

	logger.Debug(
		"performing request",
		"method", "GET",
		"url", "https://newsdata.io/api/1/latest?apikey=secret-api-key&country=nl%2Cch",
	)

	output := log.String()
	if strings.Contains(output, "secret-api-key") {
		t.Fatalf("log output leaked API key: %s", output)
	}
	if !strings.Contains(output, "apikey=********") {
		t.Fatalf("log output did not mask API key: %s", output)
	}
	if !strings.Contains(output, "country=nl%2Cch") {
		t.Fatalf("log output lost non-sensitive query values: %s", output)
	}
}
