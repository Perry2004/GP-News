package data

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
	}, 7)
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
	if query.Get("size") != "7" {
		t.Fatalf("size = %q, want %q", query.Get("size"), "7")
	}
	if query.Get("country") != "us,ca" {
		t.Fatalf("country = %q, want %q", query.Get("country"), "us,ca")
	}
	if query.Get("q") != "markets OR inflation" {
		t.Fatalf("q = %q, want %q", query.Get("q"), "markets OR inflation")
	}
}

func TestBuildNewsDataURLDefaultSize(t *testing.T) {
	requestURL, err := buildNewsDataURL("https://newsdata.test/api/1", "latest", "test-api-key", nil, 0)
	if err != nil {
		t.Fatalf("buildNewsDataURL returned error: %v", err)
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}

	if parsedURL.Query().Get("size") != "10" {
		t.Fatalf("size = %q, want %q", parsedURL.Query().Get("size"), "10")
	}
}

func TestWeightedNewsDataRegionRequestSizes(t *testing.T) {
	request := weightedNewsDataRegionRequest("europe", "Europe",
		newsDataWeightedCountryChunk{Countries: "gb,de,fr,it,es", Weight: 6},
		newsDataWeightedCountryChunk{Countries: "nl,ch,se,no,dk", Weight: 3},
		newsDataWeightedCountryChunk{Countries: "pl,be,at,ie,fi", Weight: 1},
	)

	if request.MaxArticles != newsDataBucketArticleLimit {
		t.Fatalf("MaxArticles = %d, want %d", request.MaxArticles, newsDataBucketArticleLimit)
	}

	gotSizes := make([]int, 0, len(request.Requests))
	totalSize := 0
	for _, newsRequest := range request.Requests {
		gotSizes = append(gotSizes, newsRequest.Size)
		totalSize += newsRequest.Size
	}

	wantSizes := []int{6, 3, 1}
	for i, wantSize := range wantSizes {
		if gotSizes[i] != wantSize {
			t.Fatalf("request size at index %d = %d, want %d; all sizes = %v", i, gotSizes[i], wantSize, gotSizes)
		}
	}
	if totalSize != newsDataBucketArticleLimit {
		t.Fatalf("total request size = %d, want %d", totalSize, newsDataBucketArticleLimit)
	}
}

func TestNewsDataRegionRequestsUseBucketLimit(t *testing.T) {
	for _, request := range newsDataRegionRequests() {
		if request.MaxArticles != newsDataBucketArticleLimit {
			t.Fatalf("%s MaxArticles = %d, want %d", request.ID, request.MaxArticles, newsDataBucketArticleLimit)
		}

		totalSize := 0
		for _, newsRequest := range request.Requests {
			totalSize += newsRequest.Size
		}
		if totalSize != newsDataBucketArticleLimit {
			t.Fatalf("%s total request size = %d, want %d", request.ID, totalSize, newsDataBucketArticleLimit)
		}
	}
}
