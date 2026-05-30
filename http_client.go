package main

import (
	"log/slog"
	"net/http"

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
	s.rotationLogger.Error(msg, keysAndValues...)
}

func (s *SlogAdapter) Info(msg string, keysAndValues ...interface{}) {
	s.rotationLogger.Info(msg, keysAndValues...)
}

func (s *SlogAdapter) Debug(msg string, keysAndValues ...interface{}) {
	s.rotationLogger.Debug(msg, keysAndValues...)
}

func (s *SlogAdapter) Warn(msg string, keysAndValues ...interface{}) {
	s.rotationLogger.Warn(msg, keysAndValues...)
}
