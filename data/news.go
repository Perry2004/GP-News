package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultNewsDataBaseURL            = "https://newsdata.io/api/1"
	defaultNewsDataMaxConcurrentCalls = 1
	newsDataBucketArticleLimit        = 10
	newsDataSource                    = "newsdata_io"
	newsDataFetchTimeout              = 90 * time.Second
	newsDataBucketTimeout             = 45 * time.Second
)

type NewsArticle struct {
	Title string
	Link  string
}

type NewsArticleBucket struct {
	ID       string
	Name     string
	Articles []NewsArticle
}

type NewsDataFetchFailure struct {
	BucketID string
	Error    string
}

type newsDataBucketRequest struct {
	ID          string
	Name        string
	MaxArticles int
	Requests    []newsDataRequest
}

type newsDataRequest struct {
	Endpoint string
	Params   map[string]string
	Size     int
	Weight   int
}

type newsDataWeightedCountryChunk struct {
	Countries string
	Weight    int
}

type newsDataBucketResult struct {
	Bucket   NewsArticleBucket
	Failures []NewsDataFetchFailure
}

type newsDataLimiter chan struct{}

type newsDataResponse struct {
	Status  string          `json:"status"`
	Results json.RawMessage `json:"results"`
}

type newsDataErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type newsDataArticleResponse struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

func FetchNewsDataCategoryArticles(ctx context.Context, apiKey string) ([]NewsArticleBucket, []NewsDataFetchFailure) {
	return fetchNewsDataBuckets(ctx, apiKey, newsDataCategoryRequests(), NewHTTPClient(), defaultNewsDataBaseURL)
}

func FetchNewsDataRegionArticles(ctx context.Context, apiKey string) ([]NewsArticleBucket, []NewsDataFetchFailure) {
	return fetchNewsDataBuckets(ctx, apiKey, newsDataRegionRequests(), NewHTTPClient(), defaultNewsDataBaseURL)
}

func countNewsArticles(buckets []NewsArticleBucket) int {
	count := 0
	for _, bucket := range buckets {
		count += len(bucket.Articles)
	}
	return count
}

func newsDataCategoryRequests() []newsDataBucketRequest {
	return []newsDataBucketRequest{
		{
			ID:          "markets_macro",
			Name:        "Markets & Macro",
			MaxArticles: newsDataBucketArticleLimit,
			Requests: []newsDataRequest{
				{
					Endpoint: "market",
					Params: map[string]string{
						"q": "markets OR inflation OR \"interest rates\" OR \"central bank\" OR FX",
					},
				},
			},
		},
		{
			ID:          "politics_policy",
			Name:        "Politics & Policy",
			MaxArticles: newsDataBucketArticleLimit,
			Requests: []newsDataRequest{
				{
					Endpoint: "latest",
					Params: map[string]string{
						"category": "politics",
						"q":        "policy OR regulation OR legislation OR election OR government",
					},
				},
			},
		},
		{
			ID:          "war_geopolitical_risk",
			Name:        "War & Geopolitical Risk",
			MaxArticles: newsDataBucketArticleLimit,
			Requests: []newsDataRequest{
				{
					Endpoint: "latest",
					Params: map[string]string{
						"category": "world,politics",
						"q":        "war OR conflict OR military OR sanctions OR \"national security\"",
					},
				},
			},
		},
		{
			ID:          "technology_ai",
			Name:        "Technology & AI",
			MaxArticles: newsDataBucketArticleLimit,
			Requests: []newsDataRequest{
				{
					Endpoint: "latest",
					Params: map[string]string{
						"category": "technology",
						"q":        "AI OR \"artificial intelligence\" OR semiconductor OR cybersecurity",
					},
				},
			},
		},
	}
}

func newsDataRegionRequests() []newsDataBucketRequest {
	return []newsDataBucketRequest{
		newsDataRegionRequest("us", "U.S.", "us"),
		newsDataRegionRequest("japan", "Japan", "jp"),
		newsDataRegionRequest("china_hong_kong_taiwan", "China / Hong Kong / Taiwan", "cn,hk,tw"),
		weightedNewsDataRegionRequest("asia_ex_japan", "Asia ex-Japan",
			newsDataWeightedCountryChunk{Countries: "cn,hk,tw,kr,in", Weight: 6},
			newsDataWeightedCountryChunk{Countries: "sg,id,my,th,vn", Weight: 3},
			newsDataWeightedCountryChunk{Countries: "ph,au,nz", Weight: 1},
		),
		weightedNewsDataRegionRequest("europe", "Europe",
			newsDataWeightedCountryChunk{Countries: "gb,de,fr,it,es", Weight: 6},
			newsDataWeightedCountryChunk{Countries: "nl,ch,se,no,dk", Weight: 3},
			newsDataWeightedCountryChunk{Countries: "pl,be,at,ie,fi", Weight: 1},
		),
		weightedNewsDataRegionRequest("middle_east", "Middle East",
			newsDataWeightedCountryChunk{Countries: "ae,sa,il,ir,iq", Weight: 6},
			newsDataWeightedCountryChunk{Countries: "qa,kw,om,bh,jo", Weight: 2},
			newsDataWeightedCountryChunk{Countries: "lb,eg,tr,ye,sy", Weight: 2},
		),
		newsDataRegionRequest("russia_ukraine", "Russia / Ukraine", "ru,ua"),
	}
}

func newsDataRegionRequest(id string, name string, countryChunks ...string) newsDataBucketRequest {
	weightedChunks := make([]newsDataWeightedCountryChunk, 0, len(countryChunks))
	for _, countryChunk := range countryChunks {
		weightedChunks = append(weightedChunks, newsDataWeightedCountryChunk{
			Countries: countryChunk,
			Weight:    1,
		})
	}
	return weightedNewsDataRegionRequest(id, name, weightedChunks...)
}

func weightedNewsDataRegionRequest(id string, name string, countryChunks ...newsDataWeightedCountryChunk) newsDataBucketRequest {
	requests := make([]newsDataRequest, 0, len(countryChunks))
	for _, countryChunk := range countryChunks {
		requests = append(requests, newsDataRequest{
			Endpoint: "latest",
			Params: map[string]string{
				"country": countryChunk.Countries,
			},
			Weight: countryChunk.Weight,
		})
	}
	applyWeightedNewsDataRequestSizes(requests, newsDataBucketArticleLimit)

	return newsDataBucketRequest{
		ID:          id,
		Name:        name,
		MaxArticles: newsDataBucketArticleLimit,
		Requests:    requests,
	}
}

func applyWeightedNewsDataRequestSizes(requests []newsDataRequest, maxArticles int) {
	if len(requests) == 0 || maxArticles <= 0 {
		return
	}

	totalWeight := 0
	for _, request := range requests {
		if request.Weight > 0 {
			totalWeight += request.Weight
		}
	}
	if totalWeight == 0 {
		totalWeight = len(requests)
	}

	assigned := 0
	for i := range requests {
		weight := requests[i].Weight
		if weight <= 0 {
			weight = 1
		}
		requests[i].Size = maxArticles * weight / totalWeight
		if requests[i].Size == 0 {
			requests[i].Size = 1
		}
		assigned += requests[i].Size
	}

	for assigned < maxArticles {
		for i := range requests {
			requests[i].Size++
			assigned++
			if assigned == maxArticles {
				return
			}
		}
	}
	for assigned > maxArticles {
		reduced := false
		for i := len(requests) - 1; i >= 0; i-- {
			if requests[i].Size <= 1 {
				continue
			}
			requests[i].Size--
			assigned--
			reduced = true
			if assigned == maxArticles {
				return
			}
		}
		if !reduced {
			return
		}
	}
}

func fetchNewsDataBuckets(ctx context.Context, apiKey string, bucketRequests []newsDataBucketRequest, client *http.Client, baseURL string) ([]NewsArticleBucket, []NewsDataFetchFailure) {
	fetchCtx, cancel := context.WithTimeout(ctx, newsDataFetchTimeout)
	defer cancel()

	resultsChan := make(chan newsDataBucketResult, len(bucketRequests))
	limiter := make(newsDataLimiter, defaultNewsDataMaxConcurrentCalls)

	for _, bucketRequest := range bucketRequests {
		go func(bucketRequest newsDataBucketRequest) {
			reqCtx, cancel := context.WithTimeout(fetchCtx, newsDataBucketTimeout)
			defer cancel()

			bucket, failures := fetchNewsDataBucket(reqCtx, apiKey, bucketRequest, client, baseURL, limiter)
			resultsChan <- newsDataBucketResult{
				Bucket:   bucket,
				Failures: failures,
			}
		}(bucketRequest)
	}

	buckets := make([]NewsArticleBucket, 0, len(bucketRequests))
	failures := make([]NewsDataFetchFailure, 0)
	reported := make(map[string]bool, len(bucketRequests))
	for len(reported) < len(bucketRequests) {
		select {
		case result := <-resultsChan:
			buckets = append(buckets, result.Bucket)
			failures = append(failures, result.Failures...)
			reported[result.Bucket.ID] = true
		case <-fetchCtx.Done():
			for _, bucketRequest := range bucketRequests {
				if reported[bucketRequest.ID] {
					continue
				}
				failures = append(failures, NewsDataFetchFailure{
					BucketID: bucketRequest.ID,
					Error:    fetchCtx.Err().Error(),
				})
				reported[bucketRequest.ID] = true
				slog.Warn("NewsData fetch timed out", "bucket", bucketRequest.ID, "error", fetchCtx.Err())
			}
		}
	}

	return buckets, failures
}

func fetchNewsDataBucket(ctx context.Context, apiKey string, bucketRequest newsDataBucketRequest, client *http.Client, baseURL string, limiter newsDataLimiter) (NewsArticleBucket, []NewsDataFetchFailure) {
	bucket := NewsArticleBucket{
		ID:       bucketRequest.ID,
		Name:     bucketRequest.Name,
		Articles: make([]NewsArticle, 0),
	}
	failures := make([]NewsDataFetchFailure, 0)
	seenLinks := make(map[string]bool)
	maxArticles := bucketRequest.MaxArticles
	if maxArticles <= 0 {
		maxArticles = newsDataBucketArticleLimit
	}

	for _, request := range bucketRequest.Requests {
		if len(bucket.Articles) >= maxArticles {
			break
		}

		articles, err := fetchNewsDataArticles(ctx, apiKey, request, client, baseURL, limiter)
		if err != nil {
			failures = append(failures, NewsDataFetchFailure{
				BucketID: bucketRequest.ID,
				Error:    err.Error(),
			})
			slog.Warn("NewsData fetch failed", "bucket", bucketRequest.ID, "endpoint", request.Endpoint, "error", err)
			continue
		}

		for _, article := range articles {
			normalizedLink := strings.TrimSpace(strings.ToLower(article.Link))
			if normalizedLink == "" || seenLinks[normalizedLink] {
				continue
			}
			seenLinks[normalizedLink] = true
			bucket.Articles = append(bucket.Articles, article)
			if len(bucket.Articles) >= maxArticles {
				break
			}
		}
	}

	slog.Debug("NewsData bucket fetched",
		"bucket", bucket.ID,
		"article_count", len(bucket.Articles),
		"articles", bucket.Articles,
		"source", newsDataSource,
	)
	return bucket, failures
}

func fetchNewsDataArticles(ctx context.Context, apiKey string, request newsDataRequest, client *http.Client, baseURL string, limiter newsDataLimiter) ([]NewsArticle, error) {
	if err := limiter.acquire(ctx); err != nil {
		return nil, err
	}
	defer limiter.release()

	requestURL, err := buildNewsDataURL(baseURL, request.Endpoint, apiKey, request.Params, request.Size)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed for %s: %w", request.Endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, body)
	}

	articles, err := parseNewsDataResponse(body)
	if err != nil {
		return nil, err
	}

	return articles, nil
}

func (limiter newsDataLimiter) acquire(ctx context.Context) error {
	select {
	case limiter <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (limiter newsDataLimiter) release() {
	<-limiter
}

func buildNewsDataURL(baseURL string, endpoint string, apiKey string, params map[string]string, size int) (string, error) {
	requestURL, err := url.Parse(strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpoint, "/"))
	if err != nil {
		return "", fmt.Errorf("parse NewsData URL: %w", err)
	}

	if size <= 0 {
		size = newsDataBucketArticleLimit
	}
	query := requestURL.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	query.Set("apikey", apiKey)
	query.Set("removeduplicate", "1")
	query.Set("prioritydomain", "top")
	query.Set("size", fmt.Sprintf("%d", size))
	requestURL.RawQuery = query.Encode()

	return requestURL.String(), nil
}

func parseNewsDataResponse(body []byte) ([]NewsArticle, error) {
	var response newsDataResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode NewsData response: %w: %s", err, body)
	}

	if strings.ToLower(response.Status) != "success" {
		return nil, parseNewsDataError(response.Results)
	}

	var resultArticles []newsDataArticleResponse
	if err := json.Unmarshal(response.Results, &resultArticles); err != nil {
		return nil, fmt.Errorf("decode NewsData articles: %w: %s", err, response.Results)
	}

	articles := make([]NewsArticle, 0, len(resultArticles))
	for _, resultArticle := range resultArticles {
		title := strings.TrimSpace(resultArticle.Title)
		link := strings.TrimSpace(resultArticle.Link)
		if title == "" || link == "" {
			continue
		}
		articles = append(articles, NewsArticle{
			Title: title,
			Link:  link,
		})
	}

	return articles, nil
}

func parseNewsDataError(results json.RawMessage) error {
	var errorResponse newsDataErrorResponse
	if err := json.Unmarshal(results, &errorResponse); err != nil {
		return fmt.Errorf("NewsData API error: %s", results)
	}

	if errorResponse.Code != "" && errorResponse.Message != "" {
		return fmt.Errorf("NewsData API error %s: %s", errorResponse.Code, errorResponse.Message)
	}
	if errorResponse.Message != "" {
		return fmt.Errorf("NewsData API error: %s", errorResponse.Message)
	}
	if errorResponse.Code != "" {
		return fmt.Errorf("NewsData API error %s", errorResponse.Code)
	}

	return fmt.Errorf("NewsData API error: %s", results)
}
