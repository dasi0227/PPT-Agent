package decision

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

func TestMixedAnswersPreserveZeroAndRejectMissingOrWrongAnswers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("wrong endpoint or authorization")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid request")
		}
		if len(body["questions"].(map[string]any)) != 4 {
			t.Error("mixed request was split")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-test","answers":{"zero":{"type":"noul","noul":0},"missing":{"type":"noul"},"choice":{"type":"choice","choice":"unknown","probabilities":{"yes":1,"no":0},"confidence":1},"score":{"type":"score","score":0,"legend":{"0":"Absent","1":"Present"},"probabilities":{"0":1,"1":0},"confidence":1}},"usage":{"input_tokens":10,"output_tokens":1}}`))
	}))
	defer server.Close()
	snapshot, err := NewSnapshot(&config.JevConfig{BaseURL: server.URL + "/v1/", Model: "jev-test", Key: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	response, err := snapshot.Provider.Evaluate(context.Background(), Request{State: map[string]any{"content": "sample"}, Questions: map[string]Question{
		"zero": NoulQuestion{Instructions: "Is there a problem?"}, "missing": NoulQuestion{Instructions: "Is it complete?"},
		"choice": ChoiceQuestion{Instructions: "Choose", Criteria: map[string]any{"yes": nil, "no": map[string]any{"description": "no"}}},
		"score":  ScoreQuestion{Instructions: map[string]any{"question": "Evaluate completeness", "page": "sample"}, Criteria: []any{"Absent", "Present"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if a, e := response.Noul("zero"); e != nil || a.Noul != 0 {
		t.Fatal("valid zero lost")
	}
	if _, e := response.Noul("missing"); e == nil {
		t.Fatal("missing probability became zero")
	}
	if _, e := response.Choice("choice"); e == nil {
		t.Fatal("unknown choice accepted")
	}
	if a, e := response.Score("score"); e != nil || a.Score != 0 {
		t.Fatal("zero score lost")
	}
	if _, e := response.Score("zero"); e == nil {
		t.Fatal("type mismatch accepted")
	}
}

func TestRetryAfterIsCanceledWithinCallerBudget(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
	}))
	defer server.Close()
	snapshot, _ := NewSnapshot(&config.JevConfig{BaseURL: server.URL, Model: "jev-test", Key: "test-key"})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := snapshot.Provider.Evaluate(ctx, Request{State: "sample", Questions: map[string]Question{"test": NoulQuestion{Instructions: "Is this text?"}}})
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("retry ignored budget: calls=%d err=%v", calls, err)
	}
}
