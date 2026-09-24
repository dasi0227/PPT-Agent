package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicMapsMultipartImagesAndParallelToolResults(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gateway/anthropic/v1/messages" || r.Header.Get("x-api-key") != "secret" || r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("Authorization") != "" {
			t.Error("wrong Anthropic endpoint or authentication")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		requests = append(requests, body)
		_, _ = w.Write([]byte(`{"type":"message","content":[{"type":"text","text":"first"},{"type":"text","text":"second"},{"type":"tool_use","id":"c1","name":"render_slide","input":{"slide_id":"s1"}},{"type":"tool_use","id":"c2","name":"read_ppt","input":{}}],"usage":{"input_tokens":10,"output_tokens":4,"cache_read_input_tokens":6,"cache_creation_input_tokens":2}}`))
	}))
	defer server.Close()
	a := NewAnthropicAdapter(AdapterConfig{Provider: "kimi", Model: "configured-model", APIKey: "secret", BaseURL: server.URL + "/gateway/anthropic/v1/"})
	initial := []Message{{Role: RoleSystem, Content: TextContent("policy")}, {Role: RoleUser, Content: []ContentPart{{Type: "text", Text: "before"}, {Type: "image", ImageRef: "image-ref"}, {Type: "text", Text: "after"}}}}
	resolver := &staticImageResolver{data: ImageData{Bytes: testPNG(t, 2, 2), MIMEType: "image/png"}}
	first, err := a.Generate(context.Background(), GenerateRequest{Messages: initial, ImageResolver: resolver, MaxOutputTokens: 512, Tools: []ToolSchema{{Name: "render_slide"}, {Name: "read_ppt"}}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Text() != "first\n\nsecond" || len(first.ToolCalls) != 2 || first.Usage.TotalTokens != 22 || first.Continuation != nil {
		t.Fatalf("incorrect normalization: %+v", first)
	}
	if requests[0]["max_tokens"] != float64(512) || requests[0]["thinking"].(map[string]any)["type"] != "disabled" || requests[0]["system"] != "policy" {
		t.Fatal("generation policy was lost")
	}
	parts := requests[0]["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if len(parts) != 3 || parts[0].(map[string]any)["text"] != "before" || parts[2].(map[string]any)["text"] != "after" {
		t.Fatal("multipart order changed")
	}
	image := parts[1].(map[string]any)["source"].(map[string]any)
	if image["type"] != "base64" || image["media_type"] != "image/png" || image["data"] == "" {
		t.Fatal("image was not resolved")
	}
	history := append(initial, Message{Role: RoleAssistant, Content: first.Content, ToolCalls: first.ToolCalls},
		Message{Role: RoleTool, ToolCallID: "c1", Content: []ContentPart{{Type: "text", Text: "rendered"}, {Type: "image", ImageRef: "render-ref"}}},
		Message{Role: RoleTool, ToolCallID: "c2", Content: TextContent("outline")})
	if _, err := a.Generate(context.Background(), GenerateRequest{Messages: history, ImageResolver: resolver}); err != nil {
		t.Fatal(err)
	}
	messages := requests[1]["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("tool results did not merge into one user turn: %#v", messages)
	}
	results := messages[2].(map[string]any)["content"].([]any)
	if len(results) != 2 || results[0].(map[string]any)["tool_use_id"] != "c1" || results[1].(map[string]any)["tool_use_id"] != "c2" {
		t.Fatal("parallel tool pairing was lost")
	}
	output := results[0].(map[string]any)["content"].([]any)
	if output[1].(map[string]any)["type"] != "image" {
		t.Fatal("tool screenshot was lost")
	}
	raw, _ := json.Marshal(requests)
	if strings.Contains(string(raw), "image-ref") || strings.Contains(string(raw), "render-ref") {
		t.Fatal("internal image reference leaked")
	}
}

func TestProtocolsRejectUnauthorizedImagesBeforeSending(t *testing.T) {
	for _, protocol := range []string{ProtocolResponses, ProtocolAnthropic} {
		t.Run(protocol, func(t *testing.T) {
			called := false
			s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
			defer s.Close()
			r, err := NewRegistry("model", []ProfileConfig{{Name: "model", Protocol: protocol, BaseURL: s.URL + "/v1", Model: "m", Key: "key"}})
			if err != nil {
				t.Fatal(err)
			}
			p, _ := r.Resolve("")
			_, err = p.Adapter().Generate(context.Background(), GenerateRequest{Messages: []Message{{Role: RoleUser, Content: []ContentPart{{Type: "image", ImageRef: "unauthorized"}}}}, ImageResolver: &staticImageResolver{err: errors.New("not authorized")}})
			if !errors.Is(err, ErrImageReference) || called {
				t.Fatalf("unauthorized image reached upstream: %v", err)
			}
		})
	}
}
