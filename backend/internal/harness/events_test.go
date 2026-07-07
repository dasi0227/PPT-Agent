package harness

import (
	"encoding/json"
	"testing"
)

func TestRunStartedPayloadCarriesUserInput(t *testing.T) {
	p := RunStartedPayload{RunID: "r1", Kind: "outline", Scope: "current", Mode: "normal", UserInput: "hello"}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	if got["user_input"] != "hello" {
		t.Fatalf("expected user_input=hello, got %v", got["user_input"])
	}
}

func TestRunStartedPayloadOmitEmptyUserInput(t *testing.T) {
	p := RunStartedPayload{RunID: "r1", Kind: "outline", Scope: "current", Mode: "normal"}
	raw, _ := json.Marshal(p)
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	if _, ok := got["user_input"]; ok {
		t.Fatalf("expected user_input omitted when empty, got %v", got)
	}
}
