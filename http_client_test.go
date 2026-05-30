package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSlogAdapterMasksAPIKeyInURL(t *testing.T) {
	var log bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := NewSlogAdapter(logger)

	adapter.Debug(
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
