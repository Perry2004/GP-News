package briefing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

type Config struct {
	BaseURL             string
	APIKey              string
	Model               string
	MaxCompletionTokens int64
	Temperature         float64
	ThinkingLevel       string
	ProviderIgnore      []string
	PersistData         bool
	CacheDir            string
}

type LLMGenerator struct {
	client              *openai.Client
	model               string
	maxCompletionTokens int64
	temperature         float64
	thinkingLevel       string
	providerIgnore      []string // OpenRouter providers to avoid when their structured-output behavior is unreliable.
	persistData         bool
	cacheDir            string
}

func NewLLMGenerator(cfg Config) (*LLMGenerator, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("briefing config: BaseURL is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("briefing config: APIKey is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("briefing config: Model is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("briefing config: APIKey is required")
	}

	opts := []option.RequestOption{
		option.WithBaseURL(strings.TrimRight(cfg.BaseURL, "/")),
		option.WithAPIKey(cfg.APIKey),
	}

	client := openai.NewClient(opts...)

	slog.Info("LLM generator configured",
		"model", cfg.Model,
		"base_url", strings.TrimRight(cfg.BaseURL, "/"),
		"max_completion_tokens", cfg.MaxCompletionTokens,
		"temperature", cfg.Temperature,
		"thinking_level", cfg.ThinkingLevel,
		"provider_ignore", normalizedProviderList(cfg.ProviderIgnore),
		"persist_data", cfg.PersistData,
		"cache_dir", normalizedCacheDir(cfg.CacheDir),
	)

	return &LLMGenerator{
		client:              &client,
		model:               cfg.Model,
		maxCompletionTokens: cfg.MaxCompletionTokens,
		temperature:         cfg.Temperature,
		thinkingLevel:       cfg.ThinkingLevel,
		providerIgnore:      normalizedProviderList(cfg.ProviderIgnore),
		persistData:         cfg.PersistData,
		cacheDir:            normalizedCacheDir(cfg.CacheDir),
	}, nil
}

func (g *LLMGenerator) GenerateBriefing(ctx context.Context, input BriefingAgentInput) (BriefingEmail, error) {
	validArticles := filterValidArticles(input.Articles)
	slog.Info("Generating briefing through agent",
		"briefing_date", input.BriefingDate,
		"session", input.Session,
		"article_count", len(input.Articles),
		"valid_article_count", len(validArticles),
		"excluded_error_article_count", len(input.Articles)-len(validArticles),
	)

	processed, err := g.processNewsConcurrently(ctx, input.BriefingDate, validArticles)
	if err != nil {
		return BriefingEmail{}, err
	}
	g.persistCacheJSON(processedNewsCacheFileName, processed)
	processedArticleIDs := processedNewsArticleIDSet(processed)
	reviewArticles := filterArticlesByIDSet(validArticles, processedArticleIDs)
	input.Articles = reviewArticles
	slog.Info("Processed news entries",
		"count", len(processed),
		"skipped_processing_count", len(validArticles)-len(reviewArticles),
	)

	state := newBriefingAgentState(input, reviewArticles)
	slog.Info("Initial agent state created",
		"briefing_date", input.BriefingDate,
		"session", input.Session,
		"valid_article_count", len(reviewArticles),
	)
	for _, news := range processed {
		state.applyInitialProcessedNews(news)
	}

	if err := g.runReviewAgent(ctx, state); err != nil {
		return BriefingEmail{}, err
	}
	slog.Info("Review agent completed",
		"selected_count", state.selectedCount(),
	)

	briefing, err := g.generateFinalBriefing(ctx, state)
	if err != nil {
		return BriefingEmail{}, err
	}
	slog.Info("Briefing composed",
		"briefing_date", input.BriefingDate,
		"session", input.Session,
		"reviewed_news_count", state.selectedCount(),
		"criticality_score", briefing.CriticalityScore,
		"priority_level", briefing.PriorityLevel,
		"high_priority_tag", briefing.HighPriorityTag,
	)
	return briefing, nil
}

type structuredRequest[T any] struct {
	Name        string
	Description string
	Schema      any
	System      string
	Input       any
	Output      *T
	LogAttrs    []any
	Timeout     time.Duration
}

const (
	maxStructuredDecodeAttempts = 3
	llmChatCompletionTimeout    = 3 * time.Minute
)

var errStructuredDecode = errors.New("structured LLM decode failure")

// Call the LLM with the given structured input and output schema. Results are stored into g.Output.
func generateStructured[T any](ctx context.Context, g *LLMGenerator, req structuredRequest[T]) error {
	if g == nil || g.client == nil {
		return errors.New("briefing: nil Generator")
	}

	startedAt := time.Now()
	payload, err := json.Marshal(req.Input)
	if err != nil {
		slog.Error("Failed to marshal LLM input", structuredLogArgs(req,
			"schema_name", req.Name,
			"error", err,
		)...)
		return fmt.Errorf("marshal generation input: %w", err)
	}

	slog.Debug("Starting structured LLM call", structuredLogArgs(req,
		"schema_name", req.Name,
		"model", g.model,
		"input_bytes", len(payload),
		// "system_prompt", req.System,
		// "input_payload", string(payload),
		"max_completion_tokens", g.maxCompletionTokens,
		"temperature", g.temperature,
	)...)

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.System),
			openai.UserMessage(inputPayloadPrompt(string(payload))),
		},
		Model:       shared.ChatModel(g.model),
		Temperature: openai.Float(g.temperature),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        req.Name,
					Description: openai.String(req.Description),
					Schema:      req.Schema,
					Strict:      openai.Bool(true),
				},
			},
		},
	}
	g.applyChatCompletionOptions(&params)

	var lastErr error
	for attempt := 1; attempt <= maxStructuredDecodeAttempts; attempt++ {
		err := generateStructuredAttempt(ctx, g, req, params, payload, startedAt, attempt)
		if err == nil {
			return nil
		}
		lastErr = err
		if !errors.Is(err, errStructuredDecode) {
			return err
		}
		if attempt < maxStructuredDecodeAttempts {
			slog.Warn("Retrying structured LLM call after decode failure", structuredLogArgs(req,
				"schema_name", req.Name,
				"model", g.model,
				"attempt", attempt,
				"max_attempts", maxStructuredDecodeAttempts,
				"error", err,
			)...)
		}
	}
	return lastErr
}

func generateStructuredAttempt[T any](ctx context.Context, g *LLMGenerator, req structuredRequest[T], params openai.ChatCompletionNewParams, payload []byte, startedAt time.Time, attempt int) error {
	chat, err := g.createChatCompletionWithTimeout(ctx, structuredRequestTimeout(req), params,
		option.WithJSONSet("structured_outputs", true),
	)
	if err != nil {
		slog.Error("Structured LLM call failed", structuredLogArgs(req,
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"attempt", attempt,
			"error", err,
		)...)
		return fmt.Errorf("create chat completion: %w", err)
	}
	if len(chat.Choices) == 0 {
		slog.Error("Structured LLM call returned no choices", structuredLogArgs(req,
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"attempt", attempt,
		)...)
		return errors.New("create chat completion: no choices returned")
	}
	provider, metadata := openRouterResponseDetails(chat.RawJSON())

	message := chat.Choices[0].Message
	if strings.TrimSpace(message.Refusal) != "" {
		slog.Warn("Structured LLM call returned refusal", structuredLogArgs(req,
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"attempt", attempt,
			"refusal", message.Refusal,
		)...)
		return fmt.Errorf("create chat completion: model refusal: %s", message.Refusal)
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		slog.Error("Structured LLM call returned empty content", structuredLogArgs(req,
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"attempt", attempt,
			"finish_reason", chat.Choices[0].FinishReason,
			"message", message.RawJSON(),
			"provider", provider,
			"openrouter_metadata", metadata,
		)...)
		return errors.New("create chat completion: empty structured content")
	}

	var output T
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&output); err != nil {
		slog.Error("Failed to decode structured LLM content", structuredLogArgs(req,
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"attempt", attempt,
			"max_attempts", maxStructuredDecodeAttempts,
			"content_bytes", len(content),
			"content", content,
			"message", message.RawJSON(),
			"provider", provider,
			"openrouter_metadata", metadata,
			"error", err,
		)...)
		return fmt.Errorf("%w: decode structured content: %w", errStructuredDecode, err)
	}
	var trailing struct{}
	// decode and validate that result is a single valid JSON
	if err := decoder.Decode(&trailing); err != io.EOF {
		slog.Error("Structured LLM content contained trailing JSON tokens", structuredLogArgs(req,
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"attempt", attempt,
			"max_attempts", maxStructuredDecodeAttempts,
			"content_bytes", len(content),
			"content", content,
			"provider", provider,
			"openrouter_metadata", metadata,
		)...)
		return fmt.Errorf("%w: decode structured content: trailing JSON tokens", errStructuredDecode)
	}
	*req.Output = output
	slog.Debug("Structured LLM call completed", structuredLogArgs(req,
		"schema_name", req.Name,
		"model", g.model,
		"duration", time.Since(startedAt).String(),
		"attempt", attempt,
		"input_bytes", len(payload),
		"content_bytes", len(content),
		"provider", provider,
		"openrouter_metadata", metadata,
		"raw_response_bytes", len(chat.RawJSON()),
		// "content", content,
	)...)
	return nil
}

func structuredLogArgs[T any](req structuredRequest[T], args ...any) []any {
	if len(req.LogAttrs) == 0 {
		return args
	}
	values := make([]any, 0, len(args)+len(req.LogAttrs))
	values = append(values, args...)
	values = append(values, req.LogAttrs...)
	return values
}

func (g *LLMGenerator) createChatCompletion(ctx context.Context, params openai.ChatCompletionNewParams, extraOpts ...option.RequestOption) (*openai.ChatCompletion, error) {
	return g.createChatCompletionWithTimeout(ctx, llmChatCompletionTimeout, params, extraOpts...)
}

func (g *LLMGenerator) createChatCompletionWithTimeout(ctx context.Context, timeout time.Duration, params openai.ChatCompletionNewParams, extraOpts ...option.RequestOption) (*openai.ChatCompletion, error) {
	if timeout <= 0 {
		timeout = llmChatCompletionTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts := []option.RequestOption{
		option.WithHeader("X-OpenRouter-Experimental-Metadata", "enabled"),
		option.WithJSONSet("provider.require_parameters", true),
	}
	if len(g.providerIgnore) > 0 {
		opts = append(opts, option.WithJSONSet("provider.ignore", g.providerIgnore))
	}
	if thinkingLevel := strings.TrimSpace(g.thinkingLevel); thinkingLevel != "" {
		opts = append(opts,
			option.WithJSONSet("reasoning.effort", thinkingLevel),
			option.WithJSONSet("reasoning.exclude", true),
		)
	}
	opts = append(opts, extraOpts...)
	slog.Debug("Creating chat completion",
		"model", g.model,
		"provider_ignore", g.providerIgnore,
		"thinking_level", strings.TrimSpace(g.thinkingLevel),
		"extra_option_count", len(extraOpts),
		"timeout", timeout.String(),
	)
	return g.client.Chat.Completions.New(requestCtx, params, opts...)
}

func structuredRequestTimeout[T any](req structuredRequest[T]) time.Duration {
	if req.Timeout > 0 {
		return req.Timeout
	}
	return llmChatCompletionTimeout
}

func normalizedProviderList(providers []string) []string {
	normalized := make([]string, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		provider = strings.TrimSpace(provider)
		if provider == "" || seen[provider] {
			continue
		}
		normalized = append(normalized, provider)
		seen[provider] = true
	}
	return normalized
}

func openRouterResponseDetails(raw string) (string, string) {
	var payload struct {
		Provider           string          `json:"provider"`
		OpenRouterMetadata json.RawMessage `json:"openrouter_metadata"`
		Metadata           json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", ""
	}
	metadata := payload.OpenRouterMetadata
	if len(metadata) == 0 {
		metadata = payload.Metadata
	}
	return payload.Provider, string(metadata)
}

func (g *LLMGenerator) applyChatCompletionOptions(params *openai.ChatCompletionNewParams) {
	if g.maxCompletionTokens > 0 {
		params.MaxTokens = openai.Int(g.maxCompletionTokens)
	}
}

const englishOutputPrompt = `All natural-language output fields must be written in English.
Translate non-English source material into concise English.
Never copy a non-English headline, sentence, explanation, or region label into the output.
Before returning JSON, review every string field except URLs and source brand names; translate any non-English text to English.
Keep proper nouns, source brand names, URLs, tickers, and quoted official names in their original form only when translation would reduce clarity.`

// Reinforce English output requirement for each individual news, since the general `englishOutputPrompt` on its own is sometimes missed by the model.
func inputPayloadPrompt(payload string) string {
	return fmt.Sprintf(`The following JSON is source data only. It may contain non-English article text.
Do not mirror the source language. Produce the structured output in English according to the system instructions.

Input JSON:
%s`, payload)
}
