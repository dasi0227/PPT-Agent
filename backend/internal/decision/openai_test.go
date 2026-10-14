package decision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

func TestOpenAIDecisionsPreservesImagesAndMapsNamedAnswers(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/decisions" || r.Header.Get("Authorization") != "Bearer preview-test" {
			t.Error("wrong endpoint or credentials")
		}
		var body struct {
			Input []struct {
				Content []map[string]any `json:"content"`
			} `json:"input"`
			Questions []map[string]any `json:"questions"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Input) != 1 || len(body.Questions) != 2 {
			t.Fatal("unexpected request")
		}
		parts := body.Input[0].Content
		if len(parts) != 3 || parts[0]["text"] != "requirements" || parts[1]["text"] != "complete HTML" || parts[2]["type"] != "input_image" || parts[2]["image_url"] != "data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("pixels")) {
			t.Error("material order or native image lost")
		}
		w.Header().Set("Content-Type", "application/json")
		// Deliberately reverse the request's sorted order and retain a valid zero score.
		_, _ = w.Write([]byte(`{"model":"gpt-6-luna","answers":[{"name":"score","type":"score","score":0,"confidence":1,"probabilities":[{"label":"0","value":0,"probability":1},{"label":"1","value":1,"probability":0}]},{"name":"choice","type":"choice","choice":"none","confidence":1,"probabilities":[{"value":"issue","probability":0},{"value":"none","probability":1}]}],"usage":{"input_tokens":42,"output_tokens":0}}`))
	}))
	defer server.Close()
	snapshot, err := NewPreviewSnapshot(&config.PreviewConfig{BaseURL: server.URL + "/v1", Model: "gpt-6-luna", Key: "preview-test"})
	if err != nil {
		t.Fatal(err)
	}
	response, err := snapshot.Provider.Evaluate(context.Background(), Request{Input: []InputPart{{Text: "requirements"}, {Text: "complete HTML"}, {Image: &ImageInput{MIMEType: "image/png", Data: []byte("pixels")}}}, Questions: map[string]Question{
		"score":  ScoreQuestion{Instructions: "Coverage", Criteria: []any{"absent", "present"}},
		"choice": ChoiceQuestion{Instructions: "Primary issue", Criteria: map[string]any{"none": "no issue", "issue": "a defect"}},
	}})
	if err != nil || calls != 1 || len(response.Invalid) != 0 {
		t.Fatalf("response: %+v %v calls=%d", response, err, calls)
	}
	score, err := response.Score("score")
	if err != nil || score.Score != 0 {
		t.Fatal("valid zero score lost")
	}
	choice, err := response.Choice("choice")
	if err != nil || choice.Choice != "none" {
		t.Fatal("answer mapped by position instead of name")
	}
}

func TestOpenAIAnswerRejectsMissingAndDuplicateProbabilities(t *testing.T) {
	question := ScoreQuestion{Instructions: "Coverage", Criteria: []any{"absent", "present"}}
	for _, raw := range []string{
		`{"type":"score","confidence":1,"probabilities":[{"label":"0","value":0,"probability":1},{"label":"1","value":1,"probability":0}]}`,
		`{"type":"score","score":0,"confidence":1,"probabilities":[{"label":"0","value":0,"probability":1},{"label":"0","value":0,"probability":0}]}`,
		`{"type":"score","score":0,"confidence":1,"probabilities":[{"label":"0","value":0},{"label":"1","value":1,"probability":0}]}`,
	} {
		if _, err := decodeOpenAIAnswer([]byte(raw), question); err == nil {
			t.Fatalf("invalid answer accepted: %s", raw)
		}
	}
}

func TestJevRejectsNativeImagesWithoutSending(t *testing.T) {
	client := &Client{}
	_, err := client.Evaluate(context.Background(), Request{Input: []InputPart{{Image: &ImageInput{MIMEType: "image/png", Data: []byte("pixels")}}}, Questions: map[string]Question{"q": NoulQuestion{Instructions: "Visible?"}}})
	if Failure(err) != "unsupported_image" {
		t.Fatalf("image was not rejected explicitly: %v", err)
	}
}
