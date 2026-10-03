package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type outputContractTransport struct{ handler http.Handler }

func (transport outputContractTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}

// Exercise the wire boundary, where forgetting a projection would silently hide
// all output descriptions from a model even though internal schemas look complete.
func TestAdaptersExposeOutputContractsWithoutChangingInputs(t *testing.T) {
	for _, protocol := range []string{ProtocolResponses, ProtocolAnthropic} {
		t.Run(protocol, func(t *testing.T) {
			input := map[string]any{"type": "object", "required": []any{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}}
			output := map[string]any{"type": "object", "x-content-kind": "json+image", "properties": map[string]any{
				"answers": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
					"answer": map[string]any{"type": "string", "description": "Skipped is not approval."},
				}}},
			}, "x-error-schema": map[string]any{"description": "Correct the reported field before retrying."}}
			request := GenerateRequest{Messages: []Message{{Role: RoleUser, Content: TextContent("inspect")}}, Tools: []ToolSchema{
				{Name: "inspect", Description: "Inspect the selected item.", Parameters: input, OutputSchema: output},
				{Name: "submit", Description: "Submit the result.", Parameters: map[string]any{"type": "object"}, OutputSchema: NoReplyOutput("No tool reply is sent after submission.")},
			}}
			before, _ := json.Marshal(request.Tools)
			var sentDescriptions []string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				tools := body["tools"].([]any)
				for i, raw := range tools {
					tool := raw.(map[string]any)
					if _, exists := tool["output_schema"]; exists {
						t.Error("internal output schema leaked as an unsupported API field")
					}
					description := tool["description"].(string)
					if strings.Count(description, "Output contract (") != 1 {
						t.Errorf("missing or repeated output contract: %s", description)
					}
					if i == 0 {
						for _, want := range []string{"Skipped is not approval.", "Correct the reported field before retrying.", "json+image"} {
							if !strings.Contains(description, want) {
								t.Errorf("missing output meaning %q", want)
							}
						}
						key := "parameters"
						if protocol == ProtocolAnthropic {
							key = "input_schema"
						}
						if !reflect.DeepEqual(tool[key], input) {
							t.Error("input schema changed during output projection")
						}
						sentDescriptions = append(sentDescriptions, description)
					} else if !strings.Contains(description, "No tool reply is sent after submission.") {
						t.Error("terminal output semantics missing")
					}
				}
				if protocol == ProtocolAnthropic {
					_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"done"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
				} else {
					_, _ = w.Write([]byte(`{"id":"r1","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`))
				}
			})
			registry, err := NewRegistry("model", []ProfileConfig{{Name: "model", Protocol: protocol, BaseURL: "https://adapter-test.invalid/v1", Model: "m", Key: "key"}})
			if err != nil {
				t.Fatal(err)
			}
			profile, _ := registry.Resolve("")
			provider := profile.Adapter()
			switch adapter := provider.(type) {
			case *ResponsesAdapter:
				adapter.http.client.Transport = outputContractTransport{handler: handler}
			case *AnthropicAdapter:
				adapter.http.client.Transport = outputContractTransport{handler: handler}
			default:
				t.Fatal("unexpected adapter")
			}
			for i := 0; i < 2; i++ {
				if _, err := provider.Generate(context.Background(), request); err != nil {
					t.Fatal(err)
				}
			}
			after, _ := json.Marshal(request.Tools)
			if string(before) != string(after) {
				t.Fatal("provider mutated canonical tool definitions")
			}
			if len(sentDescriptions) != 2 || sentDescriptions[0] != sentDescriptions[1] {
				t.Fatal("reusing a request changed its generated description")
			}
			if EstimateToolTokens(request.Tools) <= EstimateToolTokens([]ToolSchema{{Name: "inspect", Description: "Inspect the selected item.", Parameters: input}}) {
				t.Fatal("output documentation was omitted from context accounting")
			}
		})
	}
}
