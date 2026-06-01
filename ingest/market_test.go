package ingest

import (
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
	body := `{"chart":{"result":[{"meta":{"regularMarketPrice":5250.75,"regularMarketTime":1717000000}}],"error":null}}`
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
	if !value.Timestamp.Equal(time.Unix(1717000000, 0)) {
		t.Fatalf("Timestamp = %v, want %v", value.Timestamp, time.Unix(1717000000, 0))
	}
	if value.Source != dataSource {
		t.Fatalf("Source = %q, want %q", value.Source, dataSource)
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
