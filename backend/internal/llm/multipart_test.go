package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Check the actual HTTP payload, not just the provider-neutral messages: the
// transcript used to retain DOM comments even when the adapter dropped them.
func TestAdaptersPreserveMultipartMessagesOnWire(t *testing.T) {
	const first = `<selected_dom>{"marker_no":1,"comment":"把这里每一点复述一遍给我即可","dom_targets":[{"text_summary":"05 试跑"}]}</selected_dom>`
	const third = `<selected_dom>{"marker_no":3,"comment":"保持原文","dom_targets":[{"text_summary":"用真实任务试"}]}</selected_dom>`
	messages := []Message{
		{Role: RoleSystem, Content: []ContentPart{{Type: "text", Text: "system policy"}, {Type: "text", Text: "reference policy"}}},
		{Role: RoleUser, Content: []ContentPart{{Type: "text", Text: "回答"}, {Type: "text", Text: first}, {Type: "text", Text: third}}},
		{Role: RoleUser, Content: []ContentPart{{Type: "text", Text: first}, {Type: "text", Text: third}}},
		{Role: RoleUser, Content: []ContentPart{{Type: "text", Text: "User steering: "}, {Type: "text", Text: third}, {Type: "text", Text: first}}},
		{Role: RoleUser, Content: []ContentPart{
			{Type: "text", Text: "压缩前的要求"},
			{Type: "text", Text: `<selected_dom_reference>{"marker_no":1,"comment":"复述第一处"}</selected_dom_reference>`},
			{Type: "text", Text: `<selected_dom_reference>{"marker_no":3,"comment":"复述第三处"}</selected_dom_reference>`},
		}},
		{Role: RoleUser, Content: []ContentPart{
			{Type: "text", Text: first},
			{Type: "image", ImageRef: "project:pro_one/attachment:att_one/original"},
			{Type: "text", Text: third},
		}},
		{Role: RoleAssistant, Content: []ContentPart{{Type: "text", Text: "先读取页面"}, {Type: "text", Text: "再复述选中内容"}}, ToolCalls: []ToolCall{{ID: "call-one", Name: "read_ppt", Args: map[string]any{}}}},
		{Role: RoleTool, ToolCallID: "call-one", Content: []ContentPart{{Type: "text", Text: "页面内容"}, {Type: "text", Text: "补充定位信息"}}},
	}
	for _, name := range []string{"responses"} {
		t.Run(name, func(t *testing.T) {
			received := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				received <- body
				_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"第一段回复"},{"type":"output_text","text":"第二段回复"}]}]}`))
			}))
			defer server.Close()
			provider := NewResponsesAdapter(AdapterConfig{Provider: "custom", APIKey: "test", BaseURL: server.URL, Model: "test"})
			response, err := provider.Generate(context.Background(), GenerateRequest{
				Messages: messages,
				ImageResolver: &staticImageResolver{data: ImageData{
					Bytes: testPNG(t, 2, 2), MIMEType: "image/png",
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			body := <-received
			var wire []any
			if name == "responses" {
				wire = append(wire, map[string]any{"role": "system", "content": body["instructions"]})
				for _, value := range body["input"].([]any) {
					item := value.(map[string]any)
					if item["type"] == "function_call" {
						continue
					}
					if item["type"] == "function_call_output" {
						item = map[string]any{"role": "tool", "content": item["output"], "tool_call_id": item["call_id"]}
					}
					wire = append(wire, item)
				}
				if response.Text() != "第一段回复\n\n第二段回复" {
					t.Fatalf("multipart response was truncated: %q", response.Text())
				}
			} else {
				wire = body["messages"].([]any)
			}
			if len(wire) != len(messages) {
				t.Fatalf("wire message count = %d, want %d", len(wire), len(messages))
			}
			for i, original := range messages {
				item := wire[i].(map[string]any)
				if item["role"] != string(original.Role) {
					t.Fatalf("message %d changed role: %#v", i, item)
				}
				if original.ToolCallID != "" && item["tool_call_id"] != original.ToolCallID {
					t.Fatalf("message %d lost tool pairing: %#v", i, item)
				}
				var want []string
				for _, part := range original.Content {
					if part.Type == "text" {
						want = append(want, part.Text)
					} else if part.Type == "image" {
						want = append(want, "[image]")
					}
				}
				if got := multipartWireProjection(t, item["content"]); got != strings.Join(want, "\n\n") {
					t.Fatalf("message %d lost/reordered content:\ngot  %q\nwant %q", i, got, strings.Join(want, "\n\n"))
				}
			}
		})
	}
}

func multipartWireProjection(t *testing.T, content any) string {
	t.Helper()
	if text, ok := content.(string); ok {
		return text
	}
	parts, ok := content.([]any)
	if !ok {
		t.Fatalf("unexpected wire content: %#v", content)
	}
	var values []string
	for _, value := range parts {
		part := value.(map[string]any)
		switch part["type"] {
		case "text", "input_text", "output_text":
			values = append(values, part["text"].(string))
		case "image_url", "input_image":
			url, ok := part["image_url"].(string)
			if !ok {
				url = part["image_url"].(map[string]any)["url"].(string)
			}
			if !strings.HasPrefix(url, "data:image/png;base64,") {
				t.Fatalf("image missing or unresolved: %q", url)
			}
			values = append(values, "[image]")
		default:
			t.Fatalf("unexpected wire content part: %#v", part)
		}
	}
	return strings.Join(values, "\n\n")
}
