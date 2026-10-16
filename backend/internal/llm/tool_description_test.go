package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

type descriptionTransport struct{ handler http.Handler }

func (transport descriptionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}

func TestAdaptersSendToolDefinitionsWithoutProjectionOrMutation(t *testing.T) {
	for _, protocol := range []string{ProtocolResponses, ProtocolAnthropic} {
		t.Run(protocol, func(t *testing.T) {
			request := GenerateRequest{Messages: []Message{{Role: RoleUser, Content: TextContent("inspect")}}, Tools: []ToolSchema{tooldefinition.Get("ask_user"), tooldefinition.Get("suggest_request")}}
			request.Tools[0].Description = "Ask the user a focused question and return the answer.\n\nReturns:\n- answers: The user's answers."
			request.Tools[0].Parameters["properties"].(map[string]any)["questions"].(map[string]any)["description"] = "Questions for the user."
			before, _ := json.Marshal(request.Tools)
			calls := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				for i, raw := range body["tools"].([]any) {
					tool := raw.(map[string]any)
					if _, exists := tool["output_schema"]; exists {
						t.Error("internal output schema leaked")
					}
					description, _ := tool["description"].(string)
					if description != request.Tools[i].Description || strings.Contains(description, "Output contract (") {
						t.Error("provider changed the tool description")
					}
					key := "parameters"
					if protocol == ProtocolAnthropic {
						key = "input_schema"
					}
					var expected map[string]any
					encoded, _ := json.Marshal(request.Tools[i].Parameters)
					_ = json.Unmarshal(encoded, &expected)
					if !reflect.DeepEqual(tool[key], expected) {
						t.Error("provider changed input constraints or annotations")
					}
				}
				calls++
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
				adapter.http.client.Transport = descriptionTransport{handler: handler}
			case *AnthropicAdapter:
				adapter.http.client.Transport = descriptionTransport{handler: handler}
			}
			for i := 0; i < 2; i++ {
				if _, err := provider.Generate(context.Background(), request); err != nil {
					t.Fatal(err)
				}
			}
			after, _ := json.Marshal(request.Tools)
			if calls != 2 || string(before) != string(after) {
				t.Fatal("repeated requests mutated the canonical definitions")
			}
			short := append([]ToolSchema(nil), request.Tools...)
			short[0].Description = "Ask."
			if EstimateToolTokens(request.Tools) <= EstimateToolTokens(short) {
				t.Fatal("final description missing from token accounting")
			}
		})
	}
}
