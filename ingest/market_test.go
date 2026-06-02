package ingest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testInstrument() Instrument {
	return Instrument{
		ID:       "sp500",
		Name:     "Test sp500",
		Category: "test_category",
		Symbol:   "^GSPC",
	}
}

func TestParseYahooChartResponseSuccess(t *testing.T) {
	instrument := testInstrument()
	body := `{"chart":{"result":[{"timestamp":[1716652800,1716739200,1716825600],"meta":{"regularMarketPrice":5250.75,"regularMarketTime":1717000000},"indicators":{"quote":[{"close":[5200.0,5225.25,5250.75]}]}}],"error":null}}`
	value, err := parseYahooChartResponse(instrument, []byte(body))
	if err != nil {
		t.Fatalf("parseYahooChartResponse returned error: %v", err)
	}

	if value.ID != instrument.ID {
		t.Fatalf("ID = %q, want %q", value.ID, instrument.ID)
	}
	if value.Name != instrument.Name {
		t.Fatalf("Name = %q, want %q", value.Name, instrument.Name)
	}
	if value.Category != instrument.Category {
		t.Fatalf("Category = %q, want %q", value.Category, instrument.Category)
	}
	if value.Symbol != instrument.Symbol {
		t.Fatalf("Symbol = %q, want %q", value.Symbol, instrument.Symbol)
	}
	if value.Value != 5250.75 {
		t.Fatalf("Value = %v, want %v", value.Value, 5250.75)
	}
	if value.DailyChange != 25.5 {
		t.Fatalf("DailyChange = %v, want %v", value.DailyChange, 25.5)
	}
	wantDailyChangePercent := 25.5 / 5225.25 * 100
	if value.DailyChangePercent != wantDailyChangePercent {
		t.Fatalf("DailyChangePercent = %v, want %v", value.DailyChangePercent, wantDailyChangePercent)
	}
	if !value.DailyChangeValid {
		t.Fatal("DailyChangeValid = false, want true")
	}
	if !value.Timestamp.Equal(time.Unix(1717000000, 0)) {
		t.Fatalf("Timestamp = %v, want %v", value.Timestamp, time.Unix(1717000000, 0))
	}
	if len(value.History) != 3 {
		t.Fatalf("History length = %d, want 3", len(value.History))
	}
	if value.History[1].Close != 5225.25 {
		t.Fatalf("History[1].Close = %v, want %v", value.History[1].Close, 5225.25)
	}
	if !value.History[1].Timestamp.Equal(time.Unix(1716739200, 0)) {
		t.Fatalf("History[1].Timestamp = %v, want %v", value.History[1].Timestamp, time.Unix(1716739200, 0))
	}
	if value.Source != dataSource {
		t.Fatalf("Source = %q, want %q", value.Source, dataSource)
	}
}

func TestParseYahooChartResponseSparseHistoryLeavesDailyChangeEmpty(t *testing.T) {
	instrument := testInstrument()
	body := `{"chart":{"result":[{"timestamp":[1716652800,1716739200,1716825600],"meta":{"regularMarketPrice":5250.75,"regularMarketTime":1717000000},"indicators":{"quote":[{"close":[null,null,5250.75]}]}}],"error":null}}`
	value, err := parseYahooChartResponse(instrument, []byte(body))
	if err != nil {
		t.Fatalf("parseYahooChartResponse returned error: %v", err)
	}

	if value.Value != 5250.75 {
		t.Fatalf("Value = %v, want %v", value.Value, 5250.75)
	}
	if len(value.History) != 1 {
		t.Fatalf("History length = %d, want 1", len(value.History))
	}
	if value.DailyChange != 0 {
		t.Fatalf("DailyChange = %v, want 0", value.DailyChange)
	}
	if value.DailyChangePercent != 0 {
		t.Fatalf("DailyChangePercent = %v, want 0", value.DailyChangePercent)
	}
	if value.DailyChangeValid {
		t.Fatal("DailyChangeValid = true, want false")
	}
}

func TestParseYahooChartResponseUsesPreviousCloseBeforeLatestPoint(t *testing.T) {
	instrument := testInstrument()
	body := `{"chart":{"result":[{"timestamp":[1716652800,1716739200,1716825600],"meta":{"regularMarketPrice":5250.75,"regularMarketTime":1717000000},"indicators":{"quote":[{"close":[5200.0,5225.25,null]}]}}],"error":null}}`
	value, err := parseYahooChartResponse(instrument, []byte(body))
	if err != nil {
		t.Fatalf("parseYahooChartResponse returned error: %v", err)
	}

	if value.DailyChange != 25.5 {
		t.Fatalf("DailyChange = %v, want %v", value.DailyChange, 25.5)
	}
	if !value.DailyChangeValid {
		t.Fatal("DailyChangeValid = false, want true")
	}
}

func TestFetchYahooMarketValueRequestsFiveDayDailyHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/%5EGSPC" {
			t.Fatalf("request path = %q, want /%%5EGSPC", r.URL.EscapedPath())
		}
		if r.URL.Query().Get("range") != "5d" {
			t.Fatalf("range query = %q, want 5d", r.URL.Query().Get("range"))
		}
		if r.URL.Query().Get("interval") != "1d" {
			t.Fatalf("interval query = %q, want 1d", r.URL.Query().Get("interval"))
		}

		_, _ = fmt.Fprint(w, `{"chart":{"result":[{"timestamp":[1716652800,1716739200],"meta":{"regularMarketPrice":5250.75,"regularMarketTime":1717000000},"indicators":{"quote":[{"close":[5225.25,5250.75]}]}}],"error":null}}`)
	}))
	defer server.Close()

	value, err := fetchYahooMarketValue(context.Background(), server.Client(), server.URL, testInstrument())
	if err != nil {
		t.Fatalf("fetchYahooMarketValue returned error: %v", err)
	}
	if value.Value != 5250.75 {
		t.Fatalf("Value = %v, want 5250.75", value.Value)
	}
}

func TestLoadDataJSONLoadsOlderMarketValueCacheWithoutHistory(t *testing.T) {
	dir := t.TempDir()
	marketPath := filepath.Join(dir, "market_values.json")
	newsPath := filepath.Join(dir, "news_data.json")

	if err := os.WriteFile(marketPath, []byte(`[{"ID":"sp500","Name":"S&P 500","Category":"equity_index","Symbol":"^GSPC","Value":5250.75,"Timestamp":"2024-05-29T12:26:40Z","Source":"yahoo_chart_api"}]`), 0644); err != nil {
		t.Fatalf("write market cache: %v", err)
	}
	if err := os.WriteFile(newsPath, []byte(`{"category_buckets":[],"region_buckets":[]}`), 0644); err != nil {
		t.Fatalf("write news cache: %v", err)
	}

	marketValues, _, _, err := LoadDataJSON(marketPath, newsPath)
	if err != nil {
		t.Fatalf("LoadDataJSON returned error: %v", err)
	}
	if len(marketValues) != 1 {
		t.Fatalf("market value count = %d, want 1", len(marketValues))
	}
	if len(marketValues[0].History) != 0 {
		t.Fatalf("History length = %d, want 0", len(marketValues[0].History))
	}
	if marketValues[0].DailyChange != 0 || marketValues[0].DailyChangePercent != 0 || marketValues[0].DailyChangeValid {
		t.Fatalf("daily change fields = %v/%v/%v, want zero values", marketValues[0].DailyChange, marketValues[0].DailyChangePercent, marketValues[0].DailyChangeValid)
	}
}

func TestParseYahooChartResponseErrors(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantErrPart string
	}{
		{
			name:        "invalid JSON",
			body:        `{`,
			wantErrPart: "decode Yahoo chart response",
		},
		{
			name:        "Yahoo API error with description",
			body:        `{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found"}}}`,
			wantErrPart: "yahoo chart API error Not Found: No data found",
		},
		{
			name:        "Yahoo API error without description",
			body:        `{"chart":{"result":null,"error":{"code":"Unauthorized","description":""}}}`,
			wantErrPart: "yahoo chart API error Unauthorized",
		},
		{
			name:        "empty result",
			body:        `{"chart":{"result":[],"error":null}}`,
			wantErrPart: "yahoo chart response has no result",
		},
		{
			name:        "missing regular market price",
			body:        `{"chart":{"result":[{"meta":{"regularMarketTime":1717000000}}],"error":null}}`,
			wantErrPart: "yahoo chart response has no regular market price",
		},
		{
			name:        "missing regular market time",
			body:        `{"chart":{"result":[{"meta":{"regularMarketPrice":5250.75}}],"error":null}}`,
			wantErrPart: "yahoo chart response has no regular market time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseYahooChartResponse(testInstrument(), []byte(tt.body))
			if err == nil {
				t.Fatal("parseYahooChartResponse returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErrPart)
			}
		})
	}
}

func TestExtractYahooValue(t *testing.T) {
	price := 5250.75
	marketTime := int64(1717000000)

	value, timestamp, err := extractYahooValue(&price, &marketTime)
	if err != nil {
		t.Fatalf("extractYahooValue returned error: %v", err)
	}
	if value != price {
		t.Fatalf("value = %v, want %v", value, price)
	}
	if !timestamp.Equal(time.Unix(marketTime, 0)) {
		t.Fatalf("timestamp = %v, want %v", timestamp, time.Unix(marketTime, 0))
	}
}

func TestExtractYahooValueErrors(t *testing.T) {
	price := 5250.75
	marketTime := int64(1717000000)

	tests := []struct {
		name              string
		regularMarketTime *int64
		regularPrice      *float64
		wantErrPart       string
	}{
		{
			name:              "nil price",
			regularMarketTime: &marketTime,
			regularPrice:      nil,
			wantErrPart:       "yahoo chart response has no regular market price",
		},
		{
			name:              "nil time",
			regularMarketTime: nil,
			regularPrice:      &price,
			wantErrPart:       "yahoo chart response has no regular market time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := extractYahooValue(tt.regularPrice, tt.regularMarketTime)
			if err == nil {
				t.Fatal("extractYahooValue returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErrPart)
			}
		})
	}
}
