package briefing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/openai/openai-go"
)

const TestModel = "test-model"

func TestGenerateProcessedNewsUsesStructuredOutput(t *testing.T) {
	t.Parallel()

	var requestBody map[string]any
	var metadataHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		metadataHeader = r.Header.Get("X-OpenRouter-Experimental-Metadata")
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		writeChatContent(t, w, processedNewsJSON("a1", true, "Headline", "Summary."))
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{
		BaseURL:        server.URL,
		APIKey:         "test-key",
		Model:          TestModel,
		ProviderIgnore: []string{"akashml", "morph"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	output, err := g.GenerateProcessedNews(t.Context(), ProcessNewsInput{
		BriefingDate: "2026-05-30",
		Article: ArticleInput{
			ID:               "a1",
			Title:            "Headline",
			Link:             "https://example.test/a1",
			ExtractedContent: "The Federal Reserve discussed inflation risks.",
		},
	})
	if err != nil {
		t.Fatalf("GenerateProcessedNews() error = %v", err)
	}
	if output.ArticleID != "a1" {
		t.Fatalf("unexpected article ID %q", output.ArticleID)
	}

	if got := requestBody["model"]; got != TestModel {
		t.Fatalf("request model = %v, want %q", got, TestModel)
	}
	responseFormat, ok := requestBody["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format missing or invalid: %#v", requestBody["response_format"])
	}
	if got := responseFormat["type"]; got != "json_schema" {
		t.Fatalf("response_format.type = %v, want json_schema", got)
	}
	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("json_schema missing or invalid: %#v", responseFormat["json_schema"])
	}
	if got := jsonSchema["name"]; got != "processed_news" {
		t.Fatalf("json_schema.name = %v, want processed_news", got)
	}
	if got := jsonSchema["strict"]; got != true {
		t.Fatalf("json_schema.strict = %v, want true", got)
	}
	if got := requestBody["structured_outputs"]; got != true {
		t.Fatalf("structured_outputs = %v, want true", got)
	}
	provider, ok := requestBody["provider"].(map[string]any)
	if !ok {
		t.Fatalf("provider options missing or invalid: %#v", requestBody["provider"])
	}
	if got := provider["require_parameters"]; got != true {
		t.Fatalf("provider.require_parameters = %v, want true", got)
	}
	ignored, ok := provider["ignore"].([]any)
	if !ok {
		t.Fatalf("provider.ignore missing or invalid: %#v", provider["ignore"])
	}
	if len(ignored) != 2 || ignored[0] != "akashml" || ignored[1] != "morph" {
		t.Fatalf("provider.ignore = %#v, want [akashml morph]", ignored)
	}
	if metadataHeader != "enabled" {
		t.Fatalf("X-OpenRouter-Experimental-Metadata = %q, want enabled", metadataHeader)
	}
	if _, ok := requestBody["max_completion_tokens"]; ok {
		t.Fatalf("max_completion_tokens should be omitted when unset: %#v", requestBody["max_completion_tokens"])
	}
	if _, ok := requestBody["reasoning_effort"]; ok {
		t.Fatalf("reasoning_effort should be omitted by default: %#v", requestBody["reasoning_effort"])
	}
	messages := requestMessages(t, requestBody)
	userContent := messageContent(t, messages[1])
	if !strings.Contains(userContent, "Do not mirror the source language.") {
		t.Fatalf("user message does not contain source-language warning:\n%s", userContent)
	}
	if !strings.Contains(userContent, `"briefing_date":"2026-05-30"`) {
		t.Fatalf("user message does not contain input JSON:\n%s", userContent)
	}
}

func TestGenerateProcessedNewsUsesInputArticleID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeChatContent(t, w, processedNewsJSON("wrong-id", true, "Headline", "Summary."))
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   TestModel,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	output, err := g.GenerateProcessedNews(t.Context(), ProcessNewsInput{
		BriefingDate: "2026-05-30",
		Article:      ArticleInput{ID: "a1", Title: "Headline", Link: "https://example.test/a1"},
	})
	if err != nil {
		t.Fatalf("GenerateProcessedNews() error = %v", err)
	}
	if output.ArticleID != "a1" {
		t.Fatalf("ArticleID = %q, want input article ID a1", output.ArticleID)
	}
}

func TestGenerateProcessedNewsRejectsUnknownStructuredFields(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()

		response := map[string]any{
			"id":       "chatcmpl-test",
			"object":   "chat.completion",
			"created":  0,
			"model":    TestModel,
			"provider": "AkashML",
			"openrouter_metadata": map[string]any{
				"summary": "available=1, selected=AkashML",
			},
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"refusal": "",
						"content": `{"include":true}`,
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   TestModel,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = g.GenerateProcessedNews(t.Context(), ProcessNewsInput{
		BriefingDate: "2026-05-30",
		Article:      ArticleInput{ID: "a1", Title: "Headline", Link: "https://example.test/a1"},
	})
	if err == nil {
		t.Fatal("GenerateProcessedNews() succeeded with an unknown structured field")
	}
	if !strings.Contains(err.Error(), `unknown field "include"`) {
		t.Fatalf("GenerateProcessedNews() error = %v, want unknown include field", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if callCount != maxStructuredDecodeAttempts {
		t.Fatalf("call count = %d, want %d", callCount, maxStructuredDecodeAttempts)
	}
}

func TestGenerateProcessedNewsRetriesMalformedStructuredJSON(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		currentCall := callCount
		mu.Unlock()

		if currentCall < maxStructuredDecodeAttempts {
			writeChatContent(t, w, `{"headline":"cut off`)
			return
		}
		writeChatContent(t, w, processedNewsJSON("a1", true, "Headline", "Summary."))
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   TestModel,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	output, err := g.GenerateProcessedNews(t.Context(), ProcessNewsInput{
		BriefingDate: "2026-05-30",
		Article:      ArticleInput{ID: "a1", Title: "Headline", Link: "https://example.test/a1"},
	})
	if err != nil {
		t.Fatalf("GenerateProcessedNews() error = %v", err)
	}
	if output.Headline != "Headline" {
		t.Fatalf("headline = %q, want Headline", output.Headline)
	}
	mu.Lock()
	defer mu.Unlock()
	if callCount != maxStructuredDecodeAttempts {
		t.Fatalf("call count = %d, want %d", callCount, maxStructuredDecodeAttempts)
	}
}

func TestGenerateProcessedNewsUsesOpenRouterReasoningOptions(t *testing.T) {
	t.Parallel()

	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		writeChatContent(t, w, processedNewsJSON("a1", true, "Headline", "Summary."))
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{
		BaseURL:       server.URL,
		APIKey:        "test-key",
		Model:         TestModel,
		ThinkingLevel: "medium",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = g.GenerateProcessedNews(t.Context(), ProcessNewsInput{
		BriefingDate: "2026-05-30",
		Article:      ArticleInput{ID: "a1", Title: "Headline", Link: "https://example.test/a1"},
	})
	if err != nil {
		t.Fatalf("GenerateProcessedNews() error = %v", err)
	}

	reasoning, ok := requestBody["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("reasoning options missing or invalid: %#v", requestBody["reasoning"])
	}
	if got := reasoning["effort"]; got != "medium" {
		t.Fatalf("reasoning.effort = %v, want medium", got)
	}
	if got := reasoning["exclude"]; got != true {
		t.Fatalf("reasoning.exclude = %v, want true", got)
	}
	if _, ok := requestBody["reasoning_effort"]; ok {
		t.Fatalf("reasoning_effort should not be sent: %#v", requestBody["reasoning_effort"])
	}
}

func TestOpenRouterResponseDetails(t *testing.T) {
	t.Parallel()

	provider, metadata := openRouterResponseDetails(`{
		"provider":"AkashML",
		"openrouter_metadata":{"summary":"available=1, selected=AkashML"}
	}`)
	if provider != "AkashML" {
		t.Fatalf("provider = %q, want AkashML", provider)
	}
	if !strings.Contains(metadata, "selected=AkashML") {
		t.Fatalf("metadata = %q, want selected provider summary", metadata)
	}

	provider, metadata = openRouterResponseDetails(`{
		"provider":"Morph",
		"metadata":{"selected_provider":"Morph"}
	}`)
	if provider != "Morph" {
		t.Fatalf("provider = %q, want Morph", provider)
	}
	if !strings.Contains(metadata, "selected_provider") {
		t.Fatalf("metadata = %q, want generic metadata fallback", metadata)
	}
}

func TestReviewAgentUsesOpenRouterReasoningOptions(t *testing.T) {
	t.Parallel()

	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		writeToolCall(t, w, toolFinishReview, `{"article_ids":["a1"],"selection_rationale":"Reviewed.","global_context":"None."}`)
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{
		BaseURL:       server.URL,
		APIKey:        "test-key",
		Model:         TestModel,
		ThinkingLevel: "medium",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	state := newBriefingAgentState(BriefingAgentInput{
		BriefingDate: "2026-05-30",
		Articles:     []ArticleInput{{ID: "a1", Title: "Headline", Link: "https://example.test/a1"}},
	}, []ArticleInput{{ID: "a1", Title: "Headline", Link: "https://example.test/a1"}})
	if err := g.runReviewAgent(t.Context(), state); err != nil {
		t.Fatalf("runUnselectedReview() error = %v", err)
	}

	reasoning, ok := requestBody["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("reasoning options missing or invalid: %#v", requestBody["reasoning"])
	}
	if got := reasoning["effort"]; got != "medium" {
		t.Fatalf("reasoning.effort = %v, want medium", got)
	}
	if got := reasoning["exclude"]; got != true {
		t.Fatalf("reasoning.exclude = %v, want true", got)
	}
	if _, ok := requestBody["structured_outputs"]; ok {
		t.Fatalf("review agent should not request structured_outputs: %#v", requestBody["structured_outputs"])
	}
}

func TestGenerateBriefingFiltersErroredNewsAndForcesReviewBeforeFinal(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var processedRequests []string
	var agentRequests []map[string]any
	var finalRequests []map[string]any
	agentCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		mu.Lock()
		defer mu.Unlock()

		schemaName := responseSchemaName(requestBody)
		switch schemaName {
		case "processed_news":
			userContent := messageContent(t, requestMessages(t, requestBody)[1])
			processedRequests = append(processedRequests, userContent)
			if strings.Contains(userContent, `"id":"a1"`) {
				writeChatContent(t, w, processedNewsJSON("a1", true, "A1 headline", "A1 summary"))
				return
			}
			if strings.Contains(userContent, `"id":"a2"`) {
				writeChatContent(t, w, processedNewsJSON("a2", false, "A2 headline", "A2 summary"))
				return
			}
			t.Fatalf("unexpected processed news request:\n%s", userContent)
		case "briefing_email":
			finalRequests = append(finalRequests, requestBody)
			writeChatContent(t, w, briefingEmailJSON())
		default:
			agentCallCount++
			agentRequests = append(agentRequests, requestBody)
			if agentCallCount == 1 {
				writeChatContent(t, w, `I will draft now.`)
				return
			}
			writeToolCall(t, w, toolFinishReview, `{"article_ids":["a1","a2"],"selection_rationale":"Reviewed selected and unselected metadata.","global_context":"No extra global context."}`)
		}
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{BaseURL: server.URL, APIKey: "test-key", Model: TestModel})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = g.GenerateBriefing(t.Context(), BriefingAgentInput{
		BriefingDate: "2026-05-30",
		Session:      "Morning",
		Articles: []ArticleInput{
			{ID: "a1", BucketID: "markets", BucketName: "Markets", Title: "A1 title", Link: "https://example.test/a1", ExtractedContent: "A1 full content"},
			{ID: "a2", BucketID: "policy", BucketName: "Policy", Title: "A2 title", Link: "https://example.test/a2", ExtractedContent: "A2 full content"},
			{ID: "bad", BucketID: "bad", BucketName: "Bad", Title: "Bad title", Link: "https://example.test/bad", ExtractedContent: "Bad full content", ExtractionError: "fetch failed"},
		},
	})
	if err != nil {
		t.Fatalf("GenerateBriefing() error = %v", err)
	}

	if len(processedRequests) != 2 {
		t.Fatalf("processed request count = %d, want 2", len(processedRequests))
	}
	for _, request := range processedRequests {
		if strings.Contains(request, "Bad title") || strings.Contains(request, "Bad full content") {
			t.Fatalf("errored article leaked into processing request:\n%s", request)
		}
	}
	if len(agentRequests) != 2 {
		t.Fatalf("agent request count = %d, want 2", len(agentRequests))
	}
	if len(finalRequests) != 1 {
		t.Fatalf("final request count = %d, want 1", len(finalRequests))
	}
	if _, exists := agentRequests[0]["parallel_tool_calls"]; exists {
		t.Fatalf("parallel_tool_calls should use default behavior, got %#v", agentRequests[0]["parallel_tool_calls"])
	}
	if _, exists := agentRequests[0]["response_format"]; exists {
		t.Fatalf("review phase should not request final response schema: %#v", agentRequests[0]["response_format"])
	}
	tools, ok := agentRequests[0]["tools"].([]any)
	if !ok || len(tools) != 3 {
		t.Fatalf("agent tools missing or invalid: %#v", agentRequests[0]["tools"])
	}

	initialPrompt := messageContent(t, requestMessages(t, agentRequests[0])[1])
	if !strings.Contains(initialPrompt, "A1 summary") {
		t.Fatalf("selected summary missing from agent prompt:\n%s", initialPrompt)
	}
	if !strings.Contains(initialPrompt, "A2 title") {
		t.Fatalf("unselected metadata missing from agent prompt:\n%s", initialPrompt)
	}
	if strings.Contains(initialPrompt, "A2 summary") || strings.Contains(initialPrompt, "A2 full content") {
		t.Fatalf("unselected article summary/content leaked into agent prompt:\n%s", initialPrompt)
	}
	if strings.Contains(initialPrompt, "Bad title") || strings.Contains(initialPrompt, "Bad full content") {
		t.Fatalf("errored article leaked into agent prompt:\n%s", initialPrompt)
	}

	finalPrompt := messageContent(t, requestMessages(t, finalRequests[0])[1])
	if !strings.Contains(finalPrompt, `"reviewed_news"`) || strings.Contains(finalPrompt, `"processed_news"`) {
		t.Fatalf("final prompt should use reviewed_news handoff:\n%s", finalPrompt)
	}
	if !strings.Contains(finalPrompt, "Reviewed selected and unselected metadata.") {
		t.Fatalf("review summary missing from final prompt:\n%s", finalPrompt)
	}
}

func TestGenerateBriefingPassesReviewedNewsAndReviewSummaryToFinalComposer(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var finalRequest map[string]any
	agentCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		mu.Lock()
		defer mu.Unlock()

		switch responseSchemaName(requestBody) {
		case "processed_news":
			userContent := messageContent(t, requestMessages(t, requestBody)[1])
			if strings.Contains(userContent, `"id":"a1"`) {
				writeChatContent(t, w, processedNewsJSON("a1", true, "A1 headline", "A1 summary"))
				return
			}
			if strings.Contains(userContent, `"id":"a2"`) {
				writeChatContent(t, w, processedNewsJSON("a2", false, "A2 headline", "A2 summary"))
				return
			}
			t.Fatalf("unexpected processed news request:\n%s", userContent)
		case "briefing_email":
			finalRequest = requestBody
			writeChatContent(t, w, briefingEmailJSON())
		default:
			agentCallCount++
			switch agentCallCount {
			case 1:
				writeToolCall(t, w, toolReviewArticle, `{"article_id":"a1","include_for_briefing":false,"priority_score":0,"review_note":"Duplicate after global review.","corrections":[],"additional_context":[]}`)
			case 2:
				writeToolCall(t, w, toolReviewArticle, `{"article_id":"a2","include_for_briefing":true,"priority_score":9.5,"review_note":"Promote after review.","corrections":["Correct the policy angle."],"additional_context":["Adds regional policy context."]}`)
			default:
				writeToolCall(t, w, toolFinishReview, `{"article_ids":["a1","a2"],"selection_rationale":"Selected the stronger reviewed item.","global_context":"Policy risk is the main thread."}`)
			}
		}
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{BaseURL: server.URL, APIKey: "test-key", Model: TestModel})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = g.GenerateBriefing(t.Context(), BriefingAgentInput{
		BriefingDate: "2026-05-30",
		Session:      "Morning",
		Articles: []ArticleInput{
			{ID: "a1", BucketID: "markets", BucketName: "Markets", Title: "A1 title", Link: "https://example.test/a1", ExtractedContent: "A1 full content"},
			{ID: "a2", BucketID: "policy", BucketName: "Policy", Title: "A2 title", Link: "https://example.test/a2", ExtractedContent: "A2 full content"},
		},
	})
	if err != nil {
		t.Fatalf("GenerateBriefing() error = %v", err)
	}
	if finalRequest == nil {
		t.Fatal("final composer was not called")
	}
	finalPrompt := messageContent(t, requestMessages(t, finalRequest)[1])
	if strings.Contains(finalPrompt, "A1 summary") {
		t.Fatalf("demoted initially selected article leaked into final prompt:\n%s", finalPrompt)
	}
	for _, want := range []string{"A2 summary", "Promote after review.", "Correct the policy angle.", "Adds regional policy context.", "Selected the stronger reviewed item.", "Policy risk is the main thread."} {
		if !strings.Contains(finalPrompt, want) {
			t.Fatalf("final prompt missing %q:\n%s", want, finalPrompt)
		}
	}
}

func TestGenerateFinalBriefingRetriesInvalidNewsCardCount(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var finalRequests []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := responseSchemaName(requestBody); got != "briefing_email" {
			t.Fatalf("response schema = %q, want briefing_email", got)
		}

		mu.Lock()
		finalRequests = append(finalRequests, requestBody)
		callCount := len(finalRequests)
		mu.Unlock()

		if callCount == 1 {
			writeChatContent(t, w, briefingEmailJSONWithCardCount(20))
			return
		}
		writeChatContent(t, w, briefingEmailJSONWithCardCount(10))
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{BaseURL: server.URL, APIKey: "test-key", Model: TestModel})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	output, err := g.generateFinalBriefing(t.Context(), newBriefingAgentState(BriefingAgentInput{
		BriefingDate: "2026-05-30",
		Session:      "Morning",
	}, nil))
	if err != nil {
		t.Fatalf("generateFinalBriefing() error = %v", err)
	}
	if got := finalNewsCardCount(output); got != 10 {
		t.Fatalf("final news card count = %d, want 10", got)
	}
	if len(finalRequests) != 2 {
		t.Fatalf("final request count = %d, want 2", len(finalRequests))
	}
	retrySystemPrompt := messageContent(t, requestMessages(t, finalRequests[1])[0])
	for _, want := range []string{"previous final briefing had 20", "5 to 15 total full news card range"} {
		if !strings.Contains(retrySystemPrompt, want) {
			t.Fatalf("retry prompt missing %q:\n%s", want, retrySystemPrompt)
		}
	}
}

func TestGenerateFinalBriefingFailsAfterInvalidNewsCardCountRetry(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	finalCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := responseSchemaName(requestBody); got != "briefing_email" {
			t.Fatalf("response schema = %q, want briefing_email", got)
		}

		mu.Lock()
		finalCallCount++
		mu.Unlock()

		writeChatContent(t, w, briefingEmailJSONWithCardCount(20))
	}))
	defer server.Close()

	g, err := NewLLMGenerator(Config{BaseURL: server.URL, APIKey: "test-key", Model: TestModel})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = g.generateFinalBriefing(t.Context(), newBriefingAgentState(BriefingAgentInput{
		BriefingDate: "2026-05-30",
		Session:      "Morning",
	}, nil))
	if err == nil {
		t.Fatal("generateFinalBriefing() error = nil, want invalid card count error")
	}
	if !strings.Contains(err.Error(), "20 total full news cards") || !strings.Contains(err.Error(), "want 5 to 15") {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if finalCallCount != 2 {
		t.Fatalf("final call count = %d, want 2", finalCallCount)
	}
}

func TestAgentToolsExposeContextForAnyValidArticleAndEnforceFinishGate(t *testing.T) {
	t.Parallel()

	state := newBriefingAgentState(BriefingAgentInput{
		BriefingDate: "2026-05-30",
		Articles: []ArticleInput{
			{ID: "selected", Title: "Selected", Link: "https://example.test/selected", ExtractedContent: "Selected content"},
			{ID: "unselected", Title: "Unselected", Link: "https://example.test/unselected", ExtractedContent: "Unselected content"},
		},
	}, []ArticleInput{
		{ID: "selected", Title: "Selected", Link: "https://example.test/selected", ExtractedContent: "Selected content"},
		{ID: "unselected", Title: "Unselected", Link: "https://example.test/unselected", ExtractedContent: "Unselected content"},
	})
	state.applyInitialProcessedNews(ProcessedNews{ArticleID: "selected", Summary: "Selected summary", KeepForBriefing: true})
	state.applyInitialProcessedNews(ProcessedNews{ArticleID: "unselected", Summary: "Unselected summary", KeepForBriefing: false})

	content := state.handleGetArticleContext(`{"article_id":"unselected","include_full_content":true,"reason":"Need to inspect before selection."}`)
	if !strings.Contains(content, `"selected":false`) || !strings.Contains(content, "Unselected content") || !strings.Contains(content, "Unselected summary") {
		t.Fatalf("get_article_context did not return unselected article context: %s", content)
	}
	if state.isSelected("unselected") {
		t.Fatal("get_article_context mutated selection state")
	}

	content, complete := state.handleFinishReview(`{"article_ids":["unselected"],"selection_rationale":"done","global_context":"none"}`)
	if complete {
		t.Fatal("finish_review completed with incomplete article IDs")
	}
	if !strings.Contains(content, "must exactly match") {
		t.Fatalf("unexpected finish_review failure: %s", content)
	}

	content, complete = state.handleFinishReview(`{"article_ids":["selected","unselected"],"selection_rationale":"done","global_context":"none"}`)
	if !complete {
		t.Fatalf("finish_review did not complete with exact valid article IDs: %s", content)
	}
}

func TestReviewArticlePromotesDemotesAndStoresAnnotations(t *testing.T) {
	t.Parallel()

	article := ArticleInput{ID: "a2", Title: "A2 title", Link: "https://example.test/a2", ExtractedContent: "A2 full content"}
	state := newBriefingAgentState(BriefingAgentInput{BriefingDate: "2026-05-30", Articles: []ArticleInput{article}}, []ArticleInput{article})
	state.applyInitialProcessedNews(ProcessedNews{
		ArticleID:       "a2",
		Headline:        "A2 headline",
		Summary:         "A2 summary",
		KeepForBriefing: false,
	})

	content, err := state.handleReviewArticle(`{"article_id":"a2","include_for_briefing":false,"priority_score":1,"review_note":"Too minor.","corrections":[],"additional_context":[]}`)
	if err != nil {
		t.Fatalf("handleReviewArticle() error = %v", err)
	}
	if !strings.Contains(content, `"included":false`) {
		t.Fatalf("review_article did not report included=false: %s", content)
	}
	if state.isSelected("a2") {
		t.Fatal("a2 was selected even though include_for_briefing=false")
	}

	content, err = state.handleReviewArticle(`{"article_id":"a2","include_for_briefing":true,"priority_score":9.4,"review_note":"Include after full review.","corrections":["Use confirmed central-bank framing."],"additional_context":["Markets are focused on the rate path."]}`)
	if err != nil {
		t.Fatalf("handleReviewArticle() add error = %v", err)
	}
	if !strings.Contains(content, `"included":true`) {
		t.Fatalf("review_article did not report included=true: %s", content)
	}
	if !state.isSelected("a2") {
		t.Fatal("a2 was not added to selected news")
	}
	reviewed := state.reviewedNews()
	if len(reviewed) != 1 || reviewed[0].PriorityScore != 9.4 || reviewed[0].ReviewNote != "Include after full review." {
		t.Fatalf("review annotations not stored: %#v", reviewed)
	}
	if len(reviewed[0].Corrections) != 1 || len(reviewed[0].AdditionalContext) != 1 {
		t.Fatalf("review correction/context not stored: %#v", reviewed[0])
	}

	content, err = state.handleReviewArticle(`{"article_id":"a2","include_for_briefing":false,"priority_score":0,"review_note":"Demote after comparison.","corrections":[],"additional_context":[]}`)
	if err != nil {
		t.Fatalf("handleReviewArticle() demote error = %v", err)
	}
	if !strings.Contains(content, `"included":false`) {
		t.Fatalf("review_article did not report demotion: %s", content)
	}
	if state.isSelected("a2") {
		t.Fatal("a2 remained selected after demotion")
	}

	content, err = state.handleReviewArticle(`{"article_id":"a2","include_for_briefing":true,"priority_score":7.2,"review_note":"Re-promote after comparison.","corrections":[],"additional_context":[]}`)
	if err != nil {
		t.Fatalf("handleReviewArticle() re-promote error = %v", err)
	}
	if !strings.Contains(content, `"included":true`) {
		t.Fatalf("review_article did not report re-promotion: %s", content)
	}
	reviewed = state.reviewedNews()
	if len(reviewed) != 1 {
		t.Fatalf("reviewedNews duplicated re-promoted article: %#v", reviewed)
	}
	if reviewed[0].PriorityScore != 7.2 || reviewed[0].ReviewNote != "Re-promote after comparison." {
		t.Fatalf("reviewedNews did not keep latest re-promotion: %#v", reviewed[0])
	}
}

func TestParallelToolBatchAppliesReviewBeforeFinishReview(t *testing.T) {
	t.Parallel()

	state := newBriefingAgentState(BriefingAgentInput{
		BriefingDate: "2026-05-30",
		Articles: []ArticleInput{
			{ID: "selected", Title: "Selected", Link: "https://example.test/selected", ExtractedContent: "Selected content"},
			{ID: "unselected", Title: "Unselected", Link: "https://example.test/unselected", ExtractedContent: "Unselected content"},
		},
	}, []ArticleInput{
		{ID: "selected", Title: "Selected", Link: "https://example.test/selected", ExtractedContent: "Selected content"},
		{ID: "unselected", Title: "Unselected", Link: "https://example.test/unselected", ExtractedContent: "Unselected content"},
	})
	state.applyInitialProcessedNews(ProcessedNews{ArticleID: "selected", KeepForBriefing: true})
	state.applyInitialProcessedNews(ProcessedNews{
		ArticleID:       "unselected",
		Headline:        "Added headline",
		Summary:         "Added summary",
		KeepForBriefing: false,
	})

	results := state.handleAgentToolCalls([]openai.ChatCompletionMessageToolCall{
		toolCall("finish", toolFinishReview, `{"article_ids":["selected","unselected"],"selection_rationale":"done","global_context":"none"}`),
		toolCall("review", toolReviewArticle, `{"article_id":"unselected","include_for_briefing":true,"priority_score":8.5,"review_note":"Looks relevant.","corrections":[],"additional_context":["Adds policy context."]}`),
	})
	if len(results) != 2 {
		t.Fatalf("tool result count = %d, want 2", len(results))
	}
	for _, result := range results {
		if result.err != nil {
			t.Fatalf("tool result error = %v", result.err)
		}
	}
	if !state.isSelected("unselected") {
		t.Fatal("unselected article was not selected before finish_review was evaluated")
	}
	if !results[0].reviewComplete {
		t.Fatalf("finish_review did not complete after same-batch review_article: %s", results[0].content)
	}
	if !strings.Contains(results[1].content, `"included":true`) {
		t.Fatalf("review_article result did not report included=true: %s", results[1].content)
	}
	if state.reviewSummary.SelectionRationale != "done" || state.reviewSummary.GlobalContext != "none" {
		t.Fatalf("finish_review did not persist review summary: %#v", state.reviewSummary)
	}
}

func TestNewRequiresBaseURLAndAPIKey(t *testing.T) {
	t.Parallel()

	if _, err := NewLLMGenerator(Config{APIKey: "key"}); err == nil {
		t.Fatal("expected error for missing BaseURL")
	}
	if _, err := NewLLMGenerator(Config{BaseURL: "https://example.test"}); err == nil {
		t.Fatal("expected error for missing APIKey")
	}
}

func TestSystemPromptsRequireEnglishWithoutFormattingArtifacts(t *testing.T) {
	t.Parallel()

	for name, prompt := range map[string]string{
		"processed_news":        processedNewsSystemPrompt(),
		"briefing_agent_review": briefingAgentReviewSystemPrompt(),
		"briefing_email":        briefingSystemPrompt(),
	} {
		if !strings.Contains(prompt, "All natural-language output fields must be written in English.") {
			t.Fatalf("%s prompt does not contain English-only instruction:\n%s", name, prompt)
		}
		if !strings.Contains(prompt, "Never copy a non-English headline") {
			t.Fatalf("%s prompt does not contain non-English copying guard:\n%s", name, prompt)
		}
		if strings.Contains(prompt, "%!") {
			t.Fatalf("%s prompt contains fmt formatting artifact:\n%s", name, prompt)
		}
	}
}

func TestReviewSystemPromptRequiresToolOnlyReviewUntilFinish(t *testing.T) {
	t.Parallel()

	prompt := briefingAgentReviewSystemPrompt()
	for _, want := range []string{
		"Final briefing generation is blocked until you call finish_review",
		"including selected and unselected entries",
		"Continue the review with tool calls only",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("review prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBriefingSystemPromptDefinesFullNewsCardLimit(t *testing.T) {
	t.Parallel()

	prompt := briefingSystemPrompt()
	for _, want := range []string{
		"generate subject as a specific email subject line",
		"5 to 15 total full news cards across top_news_by_topic",
		"len(markets_macro) + len(politics_policy) + len(war_geopolitical_risk) + len(technology_ai)",
		"never exceed 15 total full news cards",
		"copy that daily_change format exactly as '+absolute (+percent%)'",
		"do not convert it to percent-only text",
		"every top_news_by_topic card must include sources as label/url objects",
		"every regional_radar item must include sources as label/url objects",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("briefing prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBriefingEmailSchemaConstrainsSubjectAndDailyChangeFormat(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(briefingEmailSchema())
	if err != nil {
		t.Fatalf("marshal briefing schema: %v", err)
	}
	schema := string(data)
	for _, want := range []string{
		"Generated email subject line for this specific briefing",
		`"minLength":12`,
		`"maxLength":120`,
		"Do not output percent-only values",
		`"pattern":"^$|^[+-][0-9]+([.][0-9]{1,2})? [(][+-][0-9]+([.][0-9]{1,2})?%[)]$"`,
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("briefing email schema missing %q:\n%s", want, schema)
		}
	}
}

func processedNewsJSON(articleID string, keep bool, headline string, summary string) string {
	data, _ := json.Marshal(ProcessedNews{
		ArticleID:            articleID,
		Headline:             headline,
		Summary:              summary,
		Entities:             []string{"Fed"},
		Region:               "U.S.",
		AssetClasses:         []string{"rates"},
		MarketRelevanceScore: 8.1,
		NoveltyScore:         6.2,
		WhyItMatters:         "Rates matter.",
		PossibleMarketImpact: "Yields may react.",
		KeepForBriefing:      keep,
		Confidence:           "High",
		SourceURL:            "https://example.test/" + articleID,
		SourceName:           "example.test",
	})
	return string(data)
}

func briefingEmailJSON() string {
	return briefingEmailJSONWithCardCount(finalNewsCardMin)
}

func briefingEmailJSONWithCardCount(cardCount int) string {
	data, _ := json.Marshal(BriefingEmail{
		Subject:             "GP News",
		CriticalityScore:    5,
		PriorityLevel:       "Watch",
		MainDriver:          "Rates",
		TodaysSignal:        "Mixed",
		ReadThisFirst:       []string{"One", "Two", "Three"},
		MacroDataWatch:      []string{},
		PolicySignalWatch:   []string{},
		TopNewsByTopic:      topNewsByTopicWithCardCount(cardCount),
		RegionalRadar:       []RegionalRadar{{Region: "Global", Sentence: "Markets are watching rates.", Sources: testBriefingSources()}},
		WatchNext:           []string{},
		WhyThisMattersToday: "Markets are watching rates.",
	})
	return string(data)
}

func testBriefingSources() []BriefingSource {
	return []BriefingSource{{Label: "example.test", URL: "https://example.test"}}
}

func topNewsByTopicWithCardCount(cardCount int) TopNewsByTopic {
	var topics TopNewsByTopic
	for i := 0; i < cardCount; i++ {
		card := NewsCard{
			Region:        "Global",
			Headline:      fmt.Sprintf("Headline %02d", i+1),
			Summary:       "Summary.",
			WhyItMatters:  "Why it matters.",
			Sources:       testBriefingSources(),
			PriorityScore: 5,
			Confidence:    "High",
		}
		switch i % 4 {
		case 0:
			card.Topic = "Markets & Macro"
			topics.MarketsMacro = append(topics.MarketsMacro, card)
		case 1:
			card.Topic = "Politics & Policy"
			topics.PoliticsPolicy = append(topics.PoliticsPolicy, card)
		case 2:
			card.Topic = "War & Geopolitical Risk"
			topics.WarGeopoliticalRisk = append(topics.WarGeopoliticalRisk, card)
		default:
			card.Topic = "Technology & AI"
			topics.TechnologyAI = append(topics.TechnologyAI, card)
		}
	}
	return topics
}

func writeChatContent(t *testing.T, w http.ResponseWriter, content string) {
	t.Helper()
	response := map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": 0,
		"model":   TestModel,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"refusal": "",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func writeToolCall(t *testing.T, w http.ResponseWriter, name string, arguments string) {
	t.Helper()
	response := map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": 0,
		"model":   TestModel,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"refusal": "",
					"content": "",
					"tool_calls": []map[string]any{
						{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      name,
								"arguments": arguments,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func toolCall(id string, name string, arguments string) openai.ChatCompletionMessageToolCall {
	var call openai.ChatCompletionMessageToolCall
	call.ID = id
	call.Function.Name = name
	call.Function.Arguments = arguments
	return call
}

func requestMessages(t *testing.T, requestBody map[string]any) []any {
	t.Helper()
	messages, ok := requestBody["messages"].([]any)
	if !ok {
		t.Fatalf("messages missing or invalid: %#v", requestBody["messages"])
	}
	return messages
}

func messageContent(t *testing.T, message any) string {
	t.Helper()
	messageMap, ok := message.(map[string]any)
	if !ok {
		t.Fatalf("message missing or invalid: %#v", message)
	}
	content, ok := messageMap["content"].(string)
	if !ok {
		t.Fatalf("message content missing or invalid: %#v", messageMap["content"])
	}
	return content
}

func responseSchemaName(requestBody map[string]any) string {
	responseFormat, ok := requestBody["response_format"].(map[string]any)
	if !ok {
		return ""
	}
	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := jsonSchema["name"].(string)
	return name
}
