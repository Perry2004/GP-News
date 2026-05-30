package main

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

func NewHTTPClient() *http.Client {
	client := retryablehttp.NewClient()
	client.Logger = NewSlogAdapter(slog.Default())
	return client.StandardClient()
}

// SlogAdapter wraps a *slog.Logger to make it compatible with retryablehttp.LeveledLogger and can be used for logging in the RetryableHTTP client.
type SlogAdapter struct {
	rotationLogger *slog.Logger
}

func NewSlogAdapter(l *slog.Logger) *SlogAdapter {
	return &SlogAdapter{rotationLogger: l}
}

func (s *SlogAdapter) Error(msg string, keysAndValues ...interface{}) {
	s.rotationLogger.Error(msg, maskLogValues(keysAndValues)...)
}

func (s *SlogAdapter) Info(msg string, keysAndValues ...interface{}) {
	s.rotationLogger.Info(msg, maskLogValues(keysAndValues)...)
}

func (s *SlogAdapter) Debug(msg string, keysAndValues ...interface{}) {
	s.rotationLogger.Debug(msg, maskLogValues(keysAndValues)...)
}

func (s *SlogAdapter) Warn(msg string, keysAndValues ...interface{}) {
	s.rotationLogger.Warn(msg, maskLogValues(keysAndValues)...)
}

func maskLogValues(keysAndValues []any) []any {
	masked := keysAndValues
	copied := false
	for i := 0; i+1 < len(masked); i += 2 {
		key, ok := masked[i].(string)
		if !ok || key != "url" {
			continue
		}

		value, ok := masked[i+1].(string)
		if !ok {
			continue
		}

		maskedURL := maskSensitiveURLQuery(value)
		if maskedURL == value {
			continue
		}

		if !copied {
			masked = append([]any(nil), keysAndValues...)
			copied = true
		}
		masked[i+1] = maskedURL
	}
	return masked
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
