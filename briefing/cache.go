package briefing

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultCacheDir                  = "cache"
	extractedNewsCacheFileName       = "extracted_news.json"
	processedNewsCacheFileName       = "processed_news.json"
	finalBriefingInputCacheFileName  = "final_briefing_input.json"
	finalBriefingDraftCacheFileName  = "final_briefing_draft.json"
	finalBriefingOutputCacheFileName = "final_briefing.json"
)

func LoadCachedBriefingEmail(cacheDir string) (BriefingEmail, error) {
	var briefing BriefingEmail
	path := cachedBriefingFilePath(cacheDir, finalBriefingOutputCacheFileName)
	if err := readCacheJSON(path, &briefing); err != nil {
		return BriefingEmail{}, fmt.Errorf("load cached briefing email %q: %w", path, err)
	}
	return briefing, nil
}

func LoadCachedProcessedNews(cacheDir string) ([]ProcessedNews, error) {
	var processed []ProcessedNews
	path := cachedBriefingFilePath(cacheDir, processedNewsCacheFileName)
	if err := readCacheJSON(path, &processed); err != nil {
		return nil, fmt.Errorf("load cached processed news %q: %w", path, err)
	}
	return processed, nil
}

func StoreExtractedNews(cacheDir string, articles []ArticleInput) error {
	path := cachedBriefingFilePath(cacheDir, extractedNewsCacheFileName)
	if err := writeCacheJSON(path, articles); err != nil {
		return fmt.Errorf("store extracted news %q: %w", path, err)
	}
	return nil
}

func LoadCachedFinalBriefingInput(cacheDir string) (BriefingInput, error) {
	var input BriefingInput
	path := cachedBriefingFilePath(cacheDir, finalBriefingInputCacheFileName)
	if err := readCacheJSON(path, &input); err != nil {
		return BriefingInput{}, fmt.Errorf("load cached final briefing input %q: %w", path, err)
	}
	return input, nil
}

func cachedBriefingFilePath(cacheDir string, fileName string) string {
	return filepath.Join(normalizedCacheDir(cacheDir), fileName)
}

func normalizedCacheDir(cacheDir string) string {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return defaultCacheDir
	}
	return cacheDir
}

func (g *LLMGenerator) persistCacheJSON(fileName string, data any) {
	if g == nil || !g.persistData {
		return
	}
	path := cachedBriefingFilePath(g.cacheDir, fileName)
	if err := writeCacheJSON(path, data); err != nil {
		slog.Error("Failed to store briefing cache JSON", "file", path, "error", err)
		return
	}
	slog.Info("Briefing cache JSON stored", "file", path)
}

func writeCacheJSON(filePath string, data any) error {
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

func readCacheJSON(filePath string, target any) error {
	jsonBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("unmarshal JSON: %w", err)
	}
	return nil
}
