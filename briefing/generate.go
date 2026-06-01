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
}

type LLMGenerator struct {
	client              *openai.Client
	model               string
	maxCompletionTokens int64
	temperature         float64
	thinkingLevel       string
	providerIgnore      []string // OpenRouter providers to avoid when their structured-output behavior is unreliable.
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
	)

	return &LLMGenerator{
		client:              &client,
		model:               cfg.Model,
		maxCompletionTokens: cfg.MaxCompletionTokens,
		temperature:         cfg.Temperature,
		thinkingLevel:       cfg.ThinkingLevel,
		providerIgnore:      normalizedProviderList(cfg.ProviderIgnore),
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

	state := newBriefingAgentState(input, validArticles)
	slog.Info("Initial agent state created",
		"briefing_date", input.BriefingDate,
		"session", input.Session,
		"valid_article_count", len(validArticles),
	)
	processed, err := g.processNewsConcurrently(ctx, input.BriefingDate, validArticles)
	slog.Info("Processed news entries", "count", len(processed))
	if err != nil {
		return BriefingEmail{}, err
	}
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
}

// Call the LLM with the given structured input and output schema. Results are stored into g.Output.
func generateStructured[T any](ctx context.Context, g *LLMGenerator, req structuredRequest[T]) error {
	if g == nil || g.client == nil {
		return errors.New("briefing: nil Generator")
	}

	startedAt := time.Now()
	payload, err := json.Marshal(req.Input)
	if err != nil {
		slog.Error("Failed to marshal LLM input",
			"schema_name", req.Name,
			"error", err,
		)
		return fmt.Errorf("marshal generation input: %w", err)
	}

	slog.Debug("Starting structured LLM call",
		"schema_name", req.Name,
		"model", g.model,
		"input_bytes", len(payload),
		// "system_prompt", req.System,
		// "input_payload", string(payload),
		"max_completion_tokens", g.maxCompletionTokens,
		"temperature", g.temperature,
	)

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
	chat, err := g.createChatCompletion(ctx, params,
		option.WithJSONSet("structured_outputs", true),
	)
	if err != nil {
		slog.Error("Structured LLM call failed",
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"error", err,
		)
		return fmt.Errorf("create chat completion: %w", err)
	}
	if len(chat.Choices) == 0 {
		slog.Error("Structured LLM call returned no choices",
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
		)
		return errors.New("create chat completion: no choices returned")
	}

	message := chat.Choices[0].Message
	if strings.TrimSpace(message.Refusal) != "" {
		slog.Warn("Structured LLM call returned refusal",
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"refusal", message.Refusal,
		)
		return fmt.Errorf("create chat completion: model refusal: %s", message.Refusal)
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		provider, metadata := openRouterResponseDetails(chat.RawJSON())
		slog.Error("Structured LLM call returned empty content",
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"finish_reason", chat.Choices[0].FinishReason,
			"message", message.RawJSON(),
			"provider", provider,
			"openrouter_metadata", metadata,
		)
		return errors.New("create chat completion: empty structured content")
	}

	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(req.Output); err != nil {
		provider, metadata := openRouterResponseDetails(chat.RawJSON())
		slog.Error("Failed to decode structured LLM content",
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"content_bytes", len(content),
			"content", content,
			"message", message.RawJSON(),
			"provider", provider,
			"openrouter_metadata", metadata,
			"error", err,
		)
		return fmt.Errorf("decode structured content: %w", err)
	}
	var trailing struct{}
	// decode and validate that result is a single valid JSON
	if err := decoder.Decode(&trailing); err != io.EOF {
		slog.Error("Structured LLM content contained trailing JSON tokens",
			"schema_name", req.Name,
			"model", g.model,
			"duration", time.Since(startedAt).String(),
			"content_bytes", len(content),
			"content", content,
		)
		return errors.New("decode structured content: trailing JSON tokens")
	}
	slog.Debug("Structured LLM call completed",
		"schema_name", req.Name,
		"model", g.model,
		"duration", time.Since(startedAt).String(),
		"input_bytes", len(payload),
		"content_bytes", len(content),
		// "content", content,
	)
	return nil
}

func (g *LLMGenerator) createChatCompletion(ctx context.Context, params openai.ChatCompletionNewParams, extraOpts ...option.RequestOption) (*openai.ChatCompletion, error) {
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
	slog.Debug("Creating chat completion with options", "options", opts)
	return g.client.Chat.Completions.New(ctx, params, opts...)
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
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", ""
	}
	return payload.Provider, string(payload.OpenRouterMetadata)
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
