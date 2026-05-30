package data

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSlogAdapterForwardsStructuredValues(t *testing.T) {
	var log bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := NewSlogAdapter(logger)

	adapter.Debug(
		"performing request",
		"method", "GET",
		"url", "https://newsdata.io/api/1/latest?apikey=secret-api-key&country=nl%2Cch",
	)

	output := log.String()
	if !strings.Contains(output, `"method":"GET"`) {
		t.Fatalf("log output did not include method: %s", output)
	}
	if !strings.Contains(output, "country=nl%2Cch") {
		t.Fatalf("log output did not include URL: %s", output)
	}
}
