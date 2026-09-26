package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsesSendsFullContextWithImagesAndClientOwnedReplay(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/responses" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("wrong Responses endpoint or authentication")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, body)
		if len(requests) == 1 {
			_, _ = w.Write([]byte(`{"id":"resp-1","output":[
				{"type":"reasoning","encrypted_content":"opaque-reasoning","summary":[]},
                {"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"checking"}]},
				{"id":"fc-1","type":"function_call","call_id":"call-1","name":"render_slide","arguments":"{\"slide_id\":\"slide-1\"}"},
				{"id":"fc-2","type":"function_call","call_id":"call-2","name":"read_resource","arguments":"{}"}
			],"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"resp-2","output":[{"type":"message","content":[{"type":"output_text","text":"done"}]}]}`))
	}))
	defer server.Close()
	adapter := NewResponsesAdapter(AdapterConfig{Provider: "openai",
		APIKey: "secret", BaseURL: server.URL + "/v1", Model: "gpt-5",
	})
	first, err := adapter.Generate(context.Background(), GenerateRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: TextContent("system policy")},
			{Role: RoleUser, Content: TextContent("make slides")},
		},
		Tools:           []ToolSchema{{Name: "render_slide"}, {Name: "read_resource"}},
		MaxOutputTokens: 512,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 2 || first.Continuation == nil || first.Usage.TotalTokens != 10 {
		t.Fatalf("Responses output was not normalized: %+v", first)
	}
	if requests[0]["max_output_tokens"] != float64(512) {
		t.Fatalf("Responses output limit missing: %#v", requests[0])
	}
	resolver := &staticImageResolver{data: ImageData{
		Bytes: testPNG(t, 32, 18), MIMEType: "image/png",
	}}
	_, err = adapter.Generate(context.Background(), GenerateRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: TextContent("system policy")},
			{Role: RoleUser, Content: TextContent("make slides")},
			{Role: RoleAssistant, Content: TextContent("checking"), ToolCalls: first.ToolCalls},
			{Role: RoleTool, ToolCallID: "call-1", Content: []ContentPart{
				{Type: "text", Text: "rendered"},
				{Type: "image", ImageRef: "run:run-1/screenshot:shot-1", Detail: "high"},
			}},
			{Role: RoleTool, ToolCallID: "call-2", Content: TextContent("outline")},
		},
		Tools:         []ToolSchema{{Name: "render_slide"}, {Name: "read_resource"}},
		ImageResolver: resolver,
		Continuation:  first.Continuation,
	})
	if err != nil {
		t.Fatal(err)
	}
	second := requests[1]
	if _, exists := second["previous_response_id"]; exists {
		t.Fatal("server-side response chaining remains")
	}
	if _, exists := second["conversation"]; exists {
		t.Fatal("server-side conversation remains")
	}
	if second["store"] != false || second["instructions"] != "system policy" {
		t.Fatalf("stateless request policy missing: %#v", second)
	}
	input := second["input"].([]any)
	if len(input) != 7 || input[0].(map[string]any)["role"] != "user" || input[1].(map[string]any)["encrypted_content"] != "opaque-reasoning" || input[2].(map[string]any)["phase"] != "commentary" {
		t.Fatalf("history or protocol replay metadata was lost: %#v", input)
	}
	firstOutput := input[5].(map[string]any)
	if firstOutput["type"] != "function_call_output" || firstOutput["call_id"] != "call-1" {
		t.Fatalf("function output pairing was lost: %#v", firstOutput)
	}
	outputParts := firstOutput["output"].([]any)
	imageURL := outputParts[1].(map[string]any)["image_url"].(string)
	if !strings.HasPrefix(imageURL, "data:image/png;base64,") ||
		strings.Contains(imageURL, "run:run-1") {
		t.Fatalf("unsafe Responses image input: %q", imageURL)
	}
	resets := 0
	_, err = adapter.Generate(context.Background(), GenerateRequest{
		Messages:            []Message{{Role: RoleSystem, Content: TextContent("new policy")}, {Role: RoleUser, Content: TextContent("compacted history")}},
		Tools:               []ToolSchema{{Name: "render_slide"}, {Name: "read_resource"}},
		Continuation:        first.Continuation,
		OnContinuationReset: func(string) { resets++ },
	})
	if err != nil || resets != 1 {
		t.Fatalf("history rewrite did not reset replay: %v resets=%d", err, resets)
	}
	if len(requests[2]["input"].([]any)) != 1 {
		t.Fatal("stale replay leaked into compacted request")
	}

}

func TestContinuationCannotCrossProviders(t *testing.T) {
	adapter := NewResponsesAdapter(AdapterConfig{Provider: "openai",
		APIKey: "secret", BaseURL: "https://api.openai.com/v1", Model: "gpt-5",
	})
	_, err := adapter.Generate(context.Background(), GenerateRequest{
		Continuation: &ProviderContinuation{
			Provider: "deepseek", Model: "deepseek-v4-pro", Opaque: json.RawMessage(`{}`),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "continuation") {
		t.Fatalf("cross-provider continuation was accepted: %v", err)
	}
}
