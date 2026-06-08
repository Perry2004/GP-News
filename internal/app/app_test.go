package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Perry2004/GP-News/briefing"
	"github.com/Perry2004/GP-News/ingest"

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

func TestConfigParsesEmailToList(t *testing.T) {
	t.Setenv("EMAIL_TO", "reader@example.com,desk@example.com")

	cfg, err := env.ParseAs[config]()
	if err != nil {
		t.Fatalf("ParseAs() error = %v", err)
	}

	if len(cfg.EmailTo) != 2 || cfg.EmailTo[0] != "reader@example.com" || cfg.EmailTo[1] != "desk@example.com" {
		t.Fatalf("EmailTo = %#v, want [reader@example.com desk@example.com]", cfg.EmailTo)
	}
	if !cfg.SendEmail {
		t.Fatal("SendEmail not defaulted to true")
	}
}

func TestConfigParsesCacheDir(t *testing.T) {
	t.Setenv("CACHE_DIR", "/tmp/gpnews-cache")

	cfg, err := env.ParseAs[config]()
	if err != nil {
		t.Fatalf("ParseAs() error = %v", err)
	}

	if cfg.CacheDir != "/tmp/gpnews-cache" {
		t.Fatalf("CacheDir = %q, want /tmp/gpnews-cache", cfg.CacheDir)
	}
}

func TestValidateConfigRequiresEmailFieldsOnlyWhenSending(t *testing.T) {
	if err := validateConfig(config{EnableFetching: false}); err != nil {
		t.Fatalf("validateConfig() with SEND_EMAIL=false error = %v", err)
	}

	tests := []struct {
		name string
		cfg  config
		want string
	}{
		{
			name: "from",
			cfg: config{
				EnableFetching: false,
				SendEmail:      true,
				EmailTo:        []string{"reader@example.com"},
			},
			want: "EMAIL_FROM is required",
		},
		{
			name: "to",
			cfg: config{
				EnableFetching: false,
				SendEmail:      true,
				EmailFrom:      "sender@example.com",
				EmailTo:        []string{"", " "},
			},
			want: "EMAIL_TO is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if err == nil {
				t.Fatal("validateConfig() returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestValidateConfigAcceptsSendEmailConfig(t *testing.T) {
	err := validateConfig(config{
		EnableFetching: false,
		SendEmail:      true,
		EmailFrom:      "sender@example.com",
		EmailTo:        []string{"reader@example.com"},
	})
	if err != nil {
		t.Fatalf("validateConfig() error = %v", err)
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

func TestBriefingTemplateDataUsesStructuredOutputShape(t *testing.T) {
	briefingEmail := testBriefingEmail()

	data, err := briefingTemplateData(briefingEmail)
	if err != nil {
		t.Fatalf("briefingTemplateData returned error: %v", err)
	}

	if data["criticality_score"] != float64(8.5) {
		t.Fatalf("criticality_score = %#v, want 8.5", data["criticality_score"])
	}
	if _, ok := data["criticalityScore"]; ok {
		t.Fatal("briefingTemplateData included camelCase criticalityScore key")
	}
	if data["full_news_card_count"] != 1 {
		t.Fatalf("full_news_card_count = %#v, want 1", data["full_news_card_count"])
	}

	marketSnapshot, ok := data["market_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("market_snapshot = %#v, want map", data["market_snapshot"])
	}
	if _, ok := marketSnapshot["equity_indices"]; !ok {
		t.Fatalf("market_snapshot missing equity_indices: %#v", marketSnapshot)
	}
	if _, ok := marketSnapshot["equityIndices"]; ok {
		t.Fatalf("market_snapshot included camelCase equityIndices: %#v", marketSnapshot)
	}

	topNewsByTopic, ok := data["top_news_by_topic"].(map[string]any)
	if !ok {
		t.Fatalf("top_news_by_topic = %#v, want map", data["top_news_by_topic"])
	}
	if _, ok := topNewsByTopic["markets_macro"]; !ok {
		t.Fatalf("top_news_by_topic missing markets_macro: %#v", topNewsByTopic)
	}
}

func TestExecuteHTMLTemplateRendersNestedBriefingData(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.html")
	outputPath := filepath.Join(dir, "rendered.html")
	templateContent := `{{.subject}}
{{range $index, $item := .read_this_first}}{{inc $index}}. {{$item}}
{{end}}
{{range .market_snapshot.equity_indices}}{{.asset}} {{.daily_change}}
{{end}}
{{range .top_news_by_topic.markets_macro}}{{.headline}} {{.why_it_matters}} {{range .sources}}{{.label}} {{.url}}{{end}}
{{end}}
{{range .regional_radar}}{{.region}} {{range .sources}}{{.label}}{{end}}{{end}}`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	data, err := briefingTemplateData(testBriefingEmail())
	if err != nil {
		t.Fatalf("briefingTemplateData returned error: %v", err)
	}

	if err := executeHTMLTemplate(templatePath, outputPath, data); err != nil {
		t.Fatalf("executeHTMLTemplate returned error: %v", err)
	}
	rendered, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read rendered output: %v", err)
	}
	output := string(rendered)
	for _, want := range []string{
		"Rates and oil drive critical briefing",
		"1. Read rates first.",
		"S&amp;P 500 &#43;25.50 (&#43;0.49%)",
		"Rates headline Rates matter.",
		"example.test https://example.test/rates",
		"Global example.test",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("rendered output missing %q:\n%s", want, output)
		}
	}
}

func TestRenderBriefingEmailHTMLWithPathsWritesCacheFile(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.html")
	outputPath := filepath.Join(dir, "cache", "briefing_email.html")
	if err := os.WriteFile(templatePath, []byte(`<html><body>{{.subject}} {{.full_news_card_count}}</body></html>`), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	renderedPath, err := renderBriefingEmailHTMLWithPaths(testBriefingEmail(), templatePath, outputPath)
	if err != nil {
		t.Fatalf("renderBriefingEmailHTMLWithPaths returned error: %v", err)
	}
	if renderedPath != outputPath {
		t.Fatalf("rendered path = %q, want %q", renderedPath, outputPath)
	}
	rendered, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read rendered output: %v", err)
	}
	if !strings.Contains(string(rendered), "Rates and oil drive critical briefing 1") {
		t.Fatalf("unexpected rendered output: %s", rendered)
	}
}

func TestRenderedEmailFilePathUsesCacheDir(t *testing.T) {
	path := renderedEmailFilePath("/tmp/gpnews-cache")
	want := filepath.Join("/tmp/gpnews-cache", "briefing_email.html")
	if path != want {
		t.Fatalf("rendered email path = %q, want %q", path, want)
	}

	defaultPath := renderedEmailFilePath("")
	if defaultPath != filepath.Join("cache", "briefing_email.html") {
		t.Fatalf("default rendered email path = %q, want cache/briefing_email.html", defaultPath)
	}
}

func TestHandleLambdaDelegatesToRun(t *testing.T) {
	original := runForLambda
	t.Cleanup(func() {
		runForLambda = original
	})

	called := false
	runForLambda = func(ctx context.Context) (Result, error) {
		called = true
		return Result{
			Status:            "ok",
			Subject:           "GP News",
			RenderedEmailPath: "cache/briefing_email.html",
		}, nil
	}

	result, err := HandleLambda(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("HandleLambda() error = %v", err)
	}
	if !called {
		t.Fatal("HandleLambda() did not call runForLambda")
	}
	if result.Subject != "GP News" {
		t.Fatalf("Subject = %q, want GP News", result.Subject)
	}
}

func TestExecuteHTMLTemplateMissingTemplateReturnsClearError(t *testing.T) {
	err := executeHTMLTemplate(filepath.Join(t.TempDir(), "missing.html"), filepath.Join(t.TempDir(), "out.html"), map[string]any{})
	if err == nil {
		t.Fatal("executeHTMLTemplate returned nil error")
	}
	if !strings.Contains(err.Error(), "read email template") {
		t.Fatalf("error = %q, want read email template", err.Error())
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

func testBriefingEmail() briefing.BriefingEmail {
	return briefing.BriefingEmail{
		Subject:          "Rates and oil drive critical briefing",
		CriticalityScore: 8.5,
		PriorityLevel:    "Critical",
		HighPriorityTag:  true,
		MainDriver:       "Rates and oil",
		TodaysSignal:     "Markets are watching rates.",
		ReadThisFirst:    []string{"Read rates first.", "Watch oil.", "Check credit."},
		MarketSnapshot: briefing.MarketSnapshot{
			EquityIndices: []briefing.MarketSnapshotItem{
				{
					Asset:       "S&P 500",
					Level:       "5250.75",
					DailyChange: "+25.50 (+0.49%)",
					Timestamp:   "2024-05-29T12:26:40Z",
					Driver:      "Rates",
					Source:      "yahoo_chart_api",
				},
			},
		},
		MacroDataWatch:    []string{"CPI"},
		PolicySignalWatch: []string{"Fed"},
		TopNewsByTopic: briefing.TopNewsByTopic{
			MarketsMacro: []briefing.NewsCard{
				{
					Topic:         "Markets & Macro",
					Region:        "Global",
					Headline:      "Rates headline",
					Summary:       "Rates summary.",
					WhyItMatters:  "Rates matter.",
					Sources:       []briefing.BriefingSource{{Label: "example.test", URL: "https://example.test/rates"}},
					PriorityScore: 8.4,
					Confidence:    "High",
					MustRead:      true,
				},
			},
		},
		RegionalRadar: []briefing.RegionalRadar{
			{
				Region:   "Global",
				Sentence: "Rates in focus.",
				Sources:  []briefing.BriefingSource{{Label: "example.test", URL: "https://example.test/radar"}},
			},
		},
		WatchNext:           []string{"Fed speakers"},
		WhyThisMattersToday: "Rates matter today.",
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
