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

func TestKimiGenerateMapsImageAndToolCalls(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"seen","tool_calls":[
			{"id":"call-1","type":"function","function":{"name":"render_slide","arguments":"{\"slide_id\":\"slide-1\"}"}},
			{"id":"call-2","type":"function","function":{"name":"read_ppt","arguments":"{}"}}
		]}}]}`))
	}))
	defer server.Close()
	adapter := NewKimiAdapter(KimiConfig{
		APIKey: "secret", BaseURL: server.URL + "/v1", Model: "kimi-k3",
	})
	resolver := &staticImageResolver{data: ImageData{
		Bytes: testPNG(t, 32, 18), MIMEType: "image/png",
	}}
	response, err := adapter.Generate(context.Background(), GenerateRequest{
		Messages: []Message{
			{Role: RoleUser, Content: TextContent("render and inspect")},
			{
				Role: RoleAssistant, ToolCalls: []ToolCall{{
					ID: "render-previous", Name: "render_slide",
					Args: map[string]any{"slide_id": "slide-1"},
				}},
			},
			{
				Role: RoleTool, ToolCallID: "render-previous",
				Content: []ContentPart{
					{Type: "text", Text: `{"slide_id":"slide-1"}`},
					{Type: "image", ImageRef: "run:run-1/screenshot:shot-1", Detail: "high"},
				},
			},
		},
		Tools:         []ToolSchema{{Name: "render_slide"}, {Name: "read_ppt"}},
		ImageResolver: resolver,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ToolCalls) != 2 || response.ToolCalls[0].ID != "call-1" ||
		response.ToolCalls[1].ID != "call-2" {
		t.Fatalf("Kimi tool calls were lost or reordered: %+v", response.ToolCalls)
	}
	messages := requestBody["messages"].([]any)
	toolResult := messages[2].(map[string]any)
	if toolResult["role"] != "tool" || toolResult["tool_call_id"] != "render-previous" ||
		toolResult["name"] != "render_slide" {
		t.Fatalf("Kimi tool result pairing was lost: %#v", toolResult)
	}
	visual := messages[3].(map[string]any)
	if visual["role"] != "user" {
		t.Fatalf("Kimi visual observation must use a user message: %#v", visual)
	}
	parts := visual["content"].([]any)
	imageURL := parts[1].(map[string]any)["image_url"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(imageURL, "data:image/png;base64,") ||
		strings.Contains(imageURL, "run:run-1") || strings.Contains(imageURL, "/Users/") {
		t.Fatalf("unsafe Kimi image payload: %q", imageURL)
	}
}

func TestKimiCannotReadUnauthorizedImageRef(t *testing.T) {
	var providerCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		providerCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	adapter := NewKimiAdapter(KimiConfig{
		APIKey: "secret", BaseURL: server.URL, Model: "kimi-k3",
	})
	_, err := adapter.Generate(context.Background(), GenerateRequest{
		Messages: []Message{{Role: RoleUser, Content: []ContentPart{{
			Type: "image", ImageRef: "run:another-run/screenshot:shot-1",
		}}}},
		ImageResolver: &staticImageResolver{err: errors.New("not authorized")},
	})
	if !errors.Is(err, ErrBadRequest) || providerCalled {
		t.Fatalf("unauthorized image reached provider: err=%v called=%v", err, providerCalled)
	}
}
