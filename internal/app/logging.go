package app

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
)

// Configures the slog logger with API key masking
func configureLogger(cfg config) error {
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
		return fmt.Errorf("invalid log level %q: expected debug, info, warn, or error", cfg.LogLevel)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: maskSensitiveLogAttr,
	}))
	slog.SetDefault(logger)
	return nil
}

func maskSensitiveLogAttr(_ []string, attr slog.Attr) slog.Attr {
	maskedValue, changed := maskLogValue(attr.Value.Any())
	if changed {
		attr.Value = slog.AnyValue(maskedValue)
	}
	return attr
}

func maskLogValue(value any) (any, bool) {
	switch value := value.(type) {
	case string:
		masked := maskSensitiveURLSubstrings(value)
		return masked, masked != value
	case error:
		masked := maskSensitiveURLSubstrings(value.Error())
		return masked, masked != value.Error()
	case fmt.Stringer:
		stringValue := value.String()
		masked := maskSensitiveURLSubstrings(stringValue)
		return masked, masked != stringValue
	default:
		return value, false
	}
}

var httpURLPattern = regexp.MustCompile(`https?://[^\s"'<>()]+`)

func maskSensitiveURLSubstrings(value string) string {
	return httpURLPattern.ReplaceAllStringFunc(value, maskSensitiveURLQuery)
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

// Returns a string of the current config with sensitive fields masked for logging.
func maskedConfig(cfg config) config {
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
