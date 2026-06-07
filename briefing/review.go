package briefing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"sort"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

const (
	// Agent tools
	toolGetArticleContext = "get_article_context"
	toolReviewArticle     = "review_article"
	toolFinishReview      = "finish_review"
)

type newsMetadata struct {
	ArticleID      string `json:"article_id"`
	BucketID       string `json:"bucket_id"`
	BucketName     string `json:"bucket_name"`
	Title          string `json:"title"`
	Link           string `json:"link"`
	Source         string `json:"source"`
	ExtractedTitle string `json:"extracted_title,omitempty"`
}

type briefingAgentPromptInput struct {
	BriefingDate   string         `json:"briefing_date"`
	Session        string         `json:"session"`
	MarketSnapshot []MarketInput  `json:"market_snapshot"`
	SelectedNews   []ReviewedNews `json:"selected_news"`
	UnselectedNews []newsMetadata `json:"unselected_news"`
}

type briefingAgentState struct {
	input         BriefingAgentInput
	articlesByID  map[string]ArticleInput
	processedByID map[string]ProcessedNews
	reviewedByID  map[string]ReviewedNews
	reviewOrder   []string
	reviewSummary ReviewSummary
}

type toolResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type getArticleContextArgs struct {
	ArticleID          string `json:"article_id"`
	IncludeFullContent bool   `json:"include_full_content"`
	Reason             string `json:"reason"`
}

type reviewArticleArgs struct {
	ArticleID          string   `json:"article_id"`
	IncludeForBriefing bool     `json:"include_for_briefing"`
	PriorityScore      float64  `json:"priority_score"`
	ReviewNote         string   `json:"review_note"`
	Corrections        []string `json:"corrections"`
	AdditionalContext  []string `json:"additional_context"`
}

type finishReviewArgs struct {
	ArticleIDs         []string `json:"article_ids"`
	SelectionRationale string   `json:"selection_rationale"`
	GlobalContext      string   `json:"global_context"`
}

type agentToolCallResult struct {
	toolCallID     string
	content        string
	reviewComplete bool
	err            error
}

func newBriefingAgentState(input BriefingAgentInput, validArticles []ArticleInput) *briefingAgentState {
	state := &briefingAgentState{
		input:         input,
		articlesByID:  make(map[string]ArticleInput, len(validArticles)),
		processedByID: make(map[string]ProcessedNews, len(validArticles)),
		reviewedByID:  make(map[string]ReviewedNews, len(validArticles)),
	}
	for _, article := range validArticles {
		state.articlesByID[article.ID] = article
	}
	return state
}

// Stores the processed news to the state and adds to selected news if it is marked as keep_for_briefing.
func (state *briefingAgentState) applyInitialProcessedNews(news ProcessedNews) {
	state.processedByID[news.ArticleID] = news
	if !news.KeepForBriefing {
		return
	}
	state.addReviewedNews(ReviewedNews{
		News:          news,
		PriorityScore: news.MarketRelevanceScore,
		ReviewNote:    "Initially selected by single-news processing.",
	})
}

func (state *briefingAgentState) addReviewedNews(reviewed ReviewedNews) {
	articleID := strings.TrimSpace(reviewed.News.ArticleID)
	if articleID == "" {
		return
	}
	if _, ok := state.articlesByID[articleID]; !ok {
		return
	}
	if _, exists := state.reviewedByID[articleID]; !exists {
		state.reviewOrder = append(state.reviewOrder, articleID)
	}
	if reviewed.Corrections == nil {
		reviewed.Corrections = []string{}
	}
	if reviewed.AdditionalContext == nil {
		reviewed.AdditionalContext = []string{}
	}
	state.reviewedByID[articleID] = reviewed
}

func (state *briefingAgentState) removeReviewedNews(articleID string) {
	delete(state.reviewedByID, strings.TrimSpace(articleID))
}

func (state *briefingAgentState) reviewedNews() []ReviewedNews {
	reviewed := make([]ReviewedNews, 0, len(state.reviewedByID))
	seen := make(map[string]bool, len(state.reviewedByID))
	for _, articleID := range state.reviewOrder {
		if seen[articleID] {
			continue
		}
		if item, ok := state.reviewedByID[articleID]; ok {
			reviewed = append(reviewed, item)
			seen[articleID] = true
		}
	}
	sort.SliceStable(reviewed, func(i, j int) bool {
		return reviewed[i].PriorityScore > reviewed[j].PriorityScore
	})
	return reviewed
}

func (state *briefingAgentState) selectedCount() int {
	return len(state.reviewedByID)
}

func (state *briefingAgentState) isSelected(articleID string) bool {
	_, ok := state.reviewedByID[articleID]
	return ok
}

func (state *briefingAgentState) isUnselected(articleID string) bool {
	if _, ok := state.articlesByID[articleID]; !ok {
		return false
	}
	return !state.isSelected(articleID)
}

func (state *briefingAgentState) validArticleIDs() map[string]bool {
	ids := make(map[string]bool, len(state.articlesByID))
	for id := range state.articlesByID {
		ids[id] = true
	}
	return ids
}

func (state *briefingAgentState) promptInput() briefingAgentPromptInput {
	return briefingAgentPromptInput{
		BriefingDate:   state.input.BriefingDate,
		Session:        state.input.Session,
		MarketSnapshot: state.input.MarketSnapshot,
		SelectedNews:   state.reviewedNews(),
		UnselectedNews: state.unselectedMetadata(),
	}
}

func (state *briefingAgentState) unselectedMetadata() []newsMetadata {
	metadata := make([]newsMetadata, 0)
	for _, article := range state.input.Articles {
		if !state.isUnselected(article.ID) {
			continue
		}
		metadata = append(metadata, metadataForArticle(article))
	}
	return metadata
}

func metadataForArticle(article ArticleInput) newsMetadata {
	return newsMetadata{
		ArticleID:      article.ID,
		BucketID:       article.BucketID,
		BucketName:     article.BucketName,
		Title:          article.Title,
		Link:           article.Link,
		Source:         sourceFromLink(article.Link),
		ExtractedTitle: article.ExtractedTitle,
	}
}

func sourceFromLink(link string) string {
	parsed, err := url.Parse(strings.TrimSpace(link))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return parsed.Hostname()
}

// Starts an agent to review and select news for the final briefing.
func (g *LLMGenerator) runReviewAgent(ctx context.Context, state *briefingAgentState) error {
	messages, err := initialAgentMessages(state)
	if err != nil {
		return err
	}

	maxIterations := len(state.articlesByID)*2 + 4
	if maxIterations < 4 {
		maxIterations = 4
	}
	slog.Debug("Starting review agent",
		"max_iterations", maxIterations,
		"valid_article_count", len(state.articlesByID),
	)
	for i := 0; i < maxIterations; i++ {
		params := openai.ChatCompletionNewParams{
			Messages:    messages,
			Model:       shared.ChatModel(g.model),
			Temperature: openai.Float(g.temperature),
			Tools:       briefingAgentTools(),
		}
		g.applyChatCompletionOptions(&params)
		slog.Debug("Sending message to review agent.",
			"iteration", i+1,
			"message_count", len(messages),
		)
		chat, err := g.createChatCompletion(ctx, params)
		if err != nil {
			return fmt.Errorf("run briefing agent review: %w", err)
		}
		if len(chat.Choices) == 0 {
			return errors.New("run briefing agent review: no choices returned")
		}

		message := chat.Choices[0].Message
		if strings.TrimSpace(message.Refusal) != "" {
			return fmt.Errorf("run briefing agent review: model refusal: %s", message.Refusal)
		}
		if len(message.ToolCalls) == 0 {
			// Reject any non-tool-call messages
			slog.Warn("Review agent response does not include any tool calls. Ignoring message and continuing review.",
				"iteration", i+1,
				"message_content", message.Content,
			)
			messages = append(messages, openai.UserMessage("Final briefing generation is blocked until you call finish_review with exactly all valid article IDs, including selected and unselected entries. Continue the review with tool calls only."))
			continue
		}

		messages = append(messages, message.ToParam())
		toolResults := state.handleAgentToolCalls(message.ToolCalls)
		reviewComplete := false
		for _, result := range toolResults {
			if result.err != nil {
				return result.err
			}
			messages = append(messages, openai.ToolMessage(result.content, result.toolCallID))
			if result.reviewComplete {
				reviewComplete = true
			}
		}
		if reviewComplete {
			state.input.Articles = filterValidArticles(state.input.Articles)
			return nil
		}
	}
	return errors.New("run briefing agent review: max iterations reached before finish_review")
}

func initialAgentMessages(state *briefingAgentState) ([]openai.ChatCompletionMessageParamUnion, error) {
	payload, err := json.Marshal(state.promptInput())
	if err != nil {
		return nil, fmt.Errorf("marshal briefing agent input: %w", err)
	}
	return []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(briefingAgentReviewSystemPrompt()),
		openai.UserMessage(inputPayloadPrompt(string(payload))),
	}, nil
}

func briefingAgentTools() []openai.ChatCompletionToolParam {
	return []openai.ChatCompletionToolParam{
		toolParam(toolGetArticleContext, "Return metadata, initial processed analysis, and optional full extracted content for any valid news entry.", map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"article_id":           map[string]any{"type": "string"},
				"include_full_content": map[string]any{"type": "boolean"},
				"reason":               map[string]any{"type": "string"},
			},
			"required": []string{"article_id", "include_full_content", "reason"},
		}),
		toolParam(toolReviewArticle, "Persist a review decision for any valid article, including promotion, demotion, priority, corrections, and additional context for final composition.", map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"article_id":           map[string]any{"type": "string"},
				"include_for_briefing": map[string]any{"type": "boolean"},
				"priority_score":       map[string]any{"type": "number"},
				"review_note":          map[string]any{"type": "string"},
				"corrections": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"additional_context": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
			"required": []string{"article_id", "include_for_briefing", "priority_score", "review_note", "corrections", "additional_context"},
		}),
		toolParam(toolFinishReview, "Finish the required review after accounting for every valid news entry and persist global review context for final composition.", map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"article_ids": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"selection_rationale": map[string]any{"type": "string"},
				"global_context":      map[string]any{"type": "string"},
			},
			"required": []string{"article_ids", "selection_rationale", "global_context"},
		}),
	}
}

func toolParam(name string, description string, parameters map[string]any) openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: shared.FunctionDefinitionParam{
			Name:        name,
			Description: openai.String(description),
			Parameters:  parameters,
			Strict:      openai.Bool(true),
		},
	}
}

func (state *briefingAgentState) handleAgentToolCalls(toolCalls []openai.ChatCompletionMessageToolCall) []agentToolCallResult {
	results := make([]agentToolCallResult, len(toolCalls))
	for i, toolCall := range toolCalls {
		results[i].toolCallID = toolCall.ID
	}

	for i, toolCall := range toolCalls {
		if toolCall.Function.Name == toolFinishReview {
			continue // finish_review tool is handled in the second loop to ensure all other tools are handled before it
		}
		content, reviewComplete, err := state.handleAgentToolCall(toolCall)
		results[i].content = content
		results[i].reviewComplete = reviewComplete
		results[i].err = err
	}
	for i, toolCall := range toolCalls {
		if toolCall.Function.Name != toolFinishReview {
			continue
		}
		content, reviewComplete, err := state.handleAgentToolCall(toolCall)
		results[i].content = content
		results[i].reviewComplete = reviewComplete
		results[i].err = err
	}
	return results
}

func (state *briefingAgentState) handleAgentToolCall(toolCall openai.ChatCompletionMessageToolCall) (string, bool, error) {
	slog.Debug("Handling review agent tool call",
		"tool_name", toolCall.Function.Name,
		"arguments", toolCall.Function.Arguments,
	)
	switch toolCall.Function.Name {
	case toolGetArticleContext:
		content := state.handleGetArticleContext(toolCall.Function.Arguments)
		return content, false, nil
	case toolReviewArticle:
		content, err := state.handleReviewArticle(toolCall.Function.Arguments)
		return content, false, err
	case toolFinishReview:
		content, complete := state.handleFinishReview(toolCall.Function.Arguments)
		return content, complete, nil
	default:
		return marshalToolResult(toolResult{OK: false, Message: fmt.Sprintf("unknown tool %q", toolCall.Function.Name)}), false, nil
	}
}

func (state *briefingAgentState) handleGetArticleContext(arguments string) string {
	var args getArticleContextArgs
	if err := decodeToolArguments(arguments, &args); err != nil {
		return marshalToolResult(toolResult{OK: false, Message: err.Error()})
	}
	articleID := strings.TrimSpace(args.ArticleID)
	article, ok := state.articlesByID[articleID]
	if !ok {
		return marshalToolResult(toolResult{OK: false, Message: "Unknown or unavailable article ID."})
	}
	news, ok := state.processedByID[articleID]
	if !ok {
		return marshalToolResult(toolResult{OK: false, Message: "Initial processed result is unavailable for this article ID."})
	}
	payload := map[string]any{
		"ok":          true,
		"article_id":  article.ID,
		"metadata":    metadataForArticle(article),
		"selected":    state.isSelected(articleID),
		"news":        news,
		"review_note": reviewedNote(state.reviewedByID[articleID]),
	}
	if args.IncludeFullContent {
		payload["extracted_content"] = article.ExtractedContent
		payload["extracted_word_count"] = article.ExtractedWordCount
	}
	return marshalToolPayload(payload)
}

func reviewedNote(reviewed ReviewedNews) string {
	return reviewed.ReviewNote
}

func (state *briefingAgentState) handleReviewArticle(arguments string) (string, error) {
	var args reviewArticleArgs
	if err := decodeToolArguments(arguments, &args); err != nil {
		return marshalToolResult(toolResult{OK: false, Message: err.Error()}), nil
	}
	articleID := strings.TrimSpace(args.ArticleID)
	if _, ok := state.articlesByID[articleID]; !ok {
		return marshalToolResult(toolResult{OK: false, Message: "Unknown or unavailable article ID."}), nil
	}

	news, ok := state.processedByID[articleID]
	if !ok {
		return marshalToolResult(toolResult{OK: false, Message: "Initial processed result is unavailable for this article ID."}), nil
	}
	if !args.IncludeForBriefing {
		state.removeReviewedNews(articleID)
		return marshalToolPayload(map[string]any{
			"ok":                   true,
			"included":             false,
			"article_id":           articleID,
			"message":              "Review decision persisted. News is excluded from final composition.",
			"selected_news_count":  state.selectedCount(),
			"reviewed_for_context": news,
		}), nil
	}
	state.addReviewedNews(ReviewedNews{
		News:              news,
		PriorityScore:     args.PriorityScore,
		ReviewNote:        args.ReviewNote,
		Corrections:       args.Corrections,
		AdditionalContext: args.AdditionalContext,
	})
	return marshalToolPayload(map[string]any{
		"ok":                  true,
		"included":            true,
		"article_id":          articleID,
		"message":             "Review decision persisted. News is included in final composition.",
		"selected_news_count": state.selectedCount(),
	}), nil
}

func (state *briefingAgentState) handleFinishReview(arguments string) (string, bool) {
	var args finishReviewArgs
	if err := decodeToolArguments(arguments, &args); err != nil {
		return marshalToolResult(toolResult{OK: false, Message: err.Error()}), false
	}
	validArticleIDs := state.validArticleIDs()
	if !sameIDSet(args.ArticleIDs, validArticleIDs) {
		return marshalToolPayload(map[string]any{
			"ok":                      false,
			"message":                 "article_ids must exactly match all valid article IDs, including selected and unselected entries.",
			"valid_article_ids":       sortedMapKeys(validArticleIDs),
			"current_unselected_ids":  state.unselectedIDs(),
			"provided_article_ids":    args.ArticleIDs,
			"selected_news_count":     state.selectedCount(),
			"remaining_review_needed": true,
		}), false
	}
	state.reviewSummary = ReviewSummary{
		SelectionRationale: args.SelectionRationale,
		GlobalContext:      args.GlobalContext,
	}
	return marshalToolPayload(map[string]any{
		"ok":                    true,
		"message":               "Review complete. Final briefing generation is now allowed.",
		"article_ids":           args.ArticleIDs,
		"selected_news_count":   state.selectedCount(),
		"unselected_news_count": len(state.unselectedIDs()),
	}), true
}

func (state *briefingAgentState) unselectedIDs() []string {
	ids := make([]string, 0)
	for _, article := range state.input.Articles {
		if state.isUnselected(article.ID) {
			ids = append(ids, article.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func sameIDSet(ids []string, expected map[string]bool) bool {
	if len(ids) != len(expected) {
		return false
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if !expected[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func sortedMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func decodeToolArguments(arguments string, output any) error {
	decoder := json.NewDecoder(strings.NewReader(arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode tool arguments: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("decode tool arguments: trailing JSON tokens")
	}
	return nil
}

func marshalToolResult(result toolResult) string {
	return marshalToolPayload(result)
}

func marshalToolPayload(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"ok":false,"message":"marshal tool payload: %s"}`, err.Error())
	}
	return string(data)
}

func briefingAgentReviewSystemPrompt() string {
	return fmt.Sprintf(`You are the GP News briefing review agent.
You are in the review phase only. You are not allowed to generate final briefing content yet.
Final briefing generation is blocked until you call finish_review with exactly all valid article IDs, including selected and unselected entries.
Continue the review with tool calls only; do not respond with ordinary assistant prose during this phase.
Review all valid news before final generation can proceed.
Use get_article_context to inspect metadata, first-pass processed analysis, and optional full content for any valid article.
Use review_article to persist every promotion, demotion, priority change, correction, or additional context that should survive into final composition.
You may remove initially selected articles by calling review_article with include_for_briefing=false.
You may promote initially unselected articles by calling review_article with include_for_briefing=true.
When every valid article has been accounted for, call finish_review with exactly all valid article IDs, plus selection_rationale and global_context for the final composer.
All natural-language tool arguments must be English.
%s`, englishOutputPrompt)
}
