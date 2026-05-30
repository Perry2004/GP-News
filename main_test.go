package main

import "testing"

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
