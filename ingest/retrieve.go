package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultCacheDir      = "cache"
	marketValuesFileName = "market_values.json"
	newsDataFileName     = "news_data.json"
)

type cachedNewsDataJSON struct {
	CategoryBuckets []NewsArticleBucket `json:"category_buckets"`
	RegionBuckets   []NewsArticleBucket `json:"region_buckets"`
}

type Config struct {
	NewsDataAPIKey string
	EnableFetching bool // whether to fetch new market and news data.
	PersistData    bool
	CacheDir       string
}

func RetrieveData(ctx context.Context, cfg Config) ([]MarketValue, []NewsArticleBucket, []NewsArticleBucket, error) {
	marketValuesFilePath := cacheFilePath(cfg.CacheDir, marketValuesFileName)
	newsDataFilePath := cacheFilePath(cfg.CacheDir, newsDataFileName)

	if !cfg.EnableFetching {
		slog.Info("Fetching disabled; loading cached data JSON", "market_file", marketValuesFilePath, "news_file", newsDataFilePath)
		marketValues, categoryBuckets, regionBuckets, err := LoadDataJSON(marketValuesFilePath, newsDataFilePath)
		if err != nil {
			return nil, nil, nil, err
		}
		// slog.Debug("Cached market values", "market_values", marketValues)
		// slog.Debug("Cached category buckets", "category_buckets", categoryBuckets)
		// slog.Debug("Cached region buckets", "region_buckets", regionBuckets)
		slog.Info("Cached data JSON loaded",
			"market_count", len(marketValues),
			"category_bucket_count", len(categoryBuckets),
			"region_bucket_count", len(regionBuckets),
			"article_count", countNewsArticles(categoryBuckets)+countNewsArticles(regionBuckets),
		)
		return marketValues, categoryBuckets, regionBuckets, nil
	}

	marketValuesChan := make(chan struct {
		Values   []MarketValue
		Failures []FetchFailure
	}, 1)

	categoryBucketsChan := make(chan struct {
		Buckets  []NewsArticleBucket
		Failures []NewsDataFetchFailure
	}, 1)

	regionBucketsChan := make(chan struct {
		Buckets  []NewsArticleBucket
		Failures []NewsDataFetchFailure
	}, 1)

	go func() {
		marketValues, fetchFailures := FetchYahooMarketValues(ctx, yahooFinanceInstruments())
		marketValuesChan <- struct {
			Values   []MarketValue
			Failures []FetchFailure
		}{Values: marketValues, Failures: fetchFailures}
	}()
	go func() {
		categoryBuckets, categoryFetchFailures := FetchNewsDataCategoryArticles(ctx, cfg.NewsDataAPIKey)
		categoryBucketsChan <- struct {
			Buckets  []NewsArticleBucket
			Failures []NewsDataFetchFailure
		}{Buckets: categoryBuckets, Failures: categoryFetchFailures}

	}()
	go func() {
		regionBuckets, regionFetchFailures := FetchNewsDataRegionArticles(ctx, cfg.NewsDataAPIKey)
		regionBucketsChan <- struct {
			Buckets  []NewsArticleBucket
			Failures []NewsDataFetchFailure
		}{Buckets: regionBuckets, Failures: regionFetchFailures}
	}()

	var marketValues []MarketValue
	var categoryBuckets []NewsArticleBucket
	var regionBuckets []NewsArticleBucket

	for i := 0; i < 3; i++ {
		select {
		case marketData := <-marketValuesChan:
			marketValues = marketData.Values
			fetchFailures := marketData.Failures
			slog.Info("Yahoo Finance market snapshot fetched", "success_count", len(marketValues), "failure_count", len(fetchFailures))
		case categoryBucketsValues := <-categoryBucketsChan:
			categoryBuckets = categoryBucketsValues.Buckets
			categoryFetchFailures := categoryBucketsValues.Failures
			slog.Info("NewsData category articles fetched", "bucket_count", len(categoryBuckets), "article_count", countNewsArticles(categoryBuckets), "failure_count", len(categoryFetchFailures))
		case regionBucketsValues := <-regionBucketsChan:
			regionBuckets = regionBucketsValues.Buckets
			regionFetchFailures := regionBucketsValues.Failures
			slog.Info("NewsData region articles fetched", "bucket_count", len(regionBuckets), "article_count", countNewsArticles(regionBuckets), "failure_count", len(regionFetchFailures))
		}
	}

	categoryBuckets, regionBuckets = enrichNewsArticleBucketsWithContent(ctx, categoryBuckets, regionBuckets)

	if cfg.PersistData {
		if err := StoreDataJSON(marketValuesFilePath, newsDataFilePath, marketValues, categoryBuckets, regionBuckets); err != nil {
			slog.Error("Failed to store data JSON", "error", err)
		} else {
			slog.Info("Data JSON files stored successfully", "market_file", marketValuesFilePath, "news_file", newsDataFilePath)
		}
	} else {
		slog.Info("Data persistence disabled; skipping JSON file storage")
	}

	return marketValues, categoryBuckets, regionBuckets, nil
}

func StoreDataJSON(marketFilePath string, newsFilePath string, marketValues []MarketValue, categoryBuckets []NewsArticleBucket, regionBuckets []NewsArticleBucket) error {
	if err := writeJSONToFile(marketFilePath, marketValues); err != nil {
		return fmt.Errorf("failed to write market values JSON: %w", err)
	}
	if err := writeJSONToFile(newsFilePath, cachedNewsDataJSON{
		CategoryBuckets: categoryBuckets,
		RegionBuckets:   regionBuckets,
	}); err != nil {
		return fmt.Errorf("failed to write news data JSON: %w", err)
	}
	return nil
}

func cacheFilePath(cacheDir string, fileName string) string {
	return filepath.Join(normalizedCacheDir(cacheDir), fileName)
}

func normalizedCacheDir(cacheDir string) string {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return defaultCacheDir
	}
	return cacheDir
}

func LoadDataJSON(marketFilePath string, newsFilePath string) ([]MarketValue, []NewsArticleBucket, []NewsArticleBucket, error) {
	var marketValues []MarketValue
	if err := readJSONFromFile(marketFilePath, &marketValues); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read market values JSON: %w", err)
	}

	var newsData cachedNewsDataJSON
	if err := readJSONFromFile(newsFilePath, &newsData); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read news data JSON: %w", err)
	}

	return marketValues, newsData.CategoryBuckets, newsData.RegionBuckets, nil
}

func writeJSONToFile(filePath string, data any) error {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func readJSONFromFile(filePath string, target any) error {
	jsonBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("unmarshal JSON: %w", err)
	}
	return nil
}
