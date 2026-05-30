package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseNewsDataResponseSuccess(t *testing.T) {
	body := `{"status":"success","results":[{"title":"  Market rally broadens  ","link":"  https://example.com/markets  "},{"title":"","link":"https://example.com/skip-title"},{"title":"Skip missing link","link":"   "}]}`

	articles, err := parseNewsDataResponse([]byte(body))
	if err != nil {
		t.Fatalf("parseNewsDataResponse returned error: %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("len(articles) = %v, want %v", len(articles), 1)
	}
	if articles[0].Title != "Market rally broadens" {
		t.Fatalf("Title = %q, want %q", articles[0].Title, "Market rally broadens")
	}
	if articles[0].Link != "https://example.com/markets" {
		t.Fatalf("Link = %q, want %q", articles[0].Link, "https://example.com/markets")
	}
}

func TestParseNewsDataResponseErrors(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantErrPart string
	}{
		{
			name:        "invalid JSON",
			body:        `{`,
			wantErrPart: "decode NewsData response",
		},
		{
			name:        "API error with code and message",
			body:        `{"status":"error","results":{"code":"Unauthorized","message":"Invalid API key"}}`,
			wantErrPart: "NewsData API error Unauthorized: Invalid API key",
		},
		{
			name:        "invalid articles JSON",
			body:        `{"status":"success","results":{}}`,
			wantErrPart: "decode NewsData articles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseNewsDataResponse([]byte(tt.body))
			if err == nil {
				t.Fatal("parseNewsDataResponse returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErrPart)
			}
		})
	}
}

func TestBuildNewsDataURL(t *testing.T) {
	requestURL, err := buildNewsDataURL("https://newsdata.test/api/1/", "/latest", "test-api-key", map[string]string{
		"country": "us,ca",
		"q":       "markets OR inflation",
	})
	if err != nil {
		t.Fatalf("buildNewsDataURL returned error: %v", err)
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}

	if parsedURL.String() == "" {
		t.Fatal("requestURL was empty")
	}
	if parsedURL.Scheme != "https" {
		t.Fatalf("Scheme = %q, want %q", parsedURL.Scheme, "https")
	}
	if parsedURL.Host != "newsdata.test" {
		t.Fatalf("Host = %q, want %q", parsedURL.Host, "newsdata.test")
	}
	if parsedURL.Path != "/api/1/latest" {
		t.Fatalf("Path = %q, want %q", parsedURL.Path, "/api/1/latest")
	}

	query := parsedURL.Query()
	if query.Get("apikey") != "test-api-key" {
		t.Fatalf("apikey = %q, want %q", query.Get("apikey"), "test-api-key")
	}
	if query.Get("removeduplicate") != "1" {
		t.Fatalf("removeduplicate = %q, want %q", query.Get("removeduplicate"), "1")
	}
	if query.Get("prioritydomain") != "top" {
		t.Fatalf("prioritydomain = %q, want %q", query.Get("prioritydomain"), "top")
	}
	if query.Get("size") != "10" {
		t.Fatalf("size = %q, want %q", query.Get("size"), "10")
	}
	if query.Get("country") != "us,ca" {
		t.Fatalf("country = %q, want %q", query.Get("country"), "us,ca")
	}
	if query.Get("q") != "markets OR inflation" {
		t.Fatalf("q = %q, want %q", query.Get("q"), "markets OR inflation")
	}
}
