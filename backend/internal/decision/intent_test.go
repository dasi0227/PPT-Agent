package decision

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

type intentProvider func(context.Context, Request) (Response, error)

func (f intentProvider) Evaluate(ctx context.Context, req Request) (Response, error) {
	return f(ctx, req)
}

func TestIntentContextKeepsFullRulesAndOrderedDialogue(t *testing.T) {
	history := []model.DialogueRound{{UserInputs: []string{"之前的请求", "追加要求"}, FinalReply: "最终回复原文"}}
	req, fingerprint, err := IntentRequest(history, "翻译一下")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(req.State)
	if err != nil {
		t.Fatal(err)
	}
	var blocks []struct {
		Kind    string
		Content json.RawMessage
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	for _, block := range blocks {
		kinds = append(kinds, block.Kind)
	}
	if !reflect.DeepEqual(kinds, []string{"agent_rules", "output_rules", "recent_four_dialogue_rounds", "classification_task", "current_user_input"}) {
		t.Fatalf("context order: %v", kinds)
	}
	for i, id := range []string{"core.agent", "core.output"} {
		var body string
		_ = json.Unmarshal(blocks[i].Content, &body)
		if body != prompt.MustLoad(id).Body {
			t.Fatalf("%s was rewritten or truncated", id)
		}
	}
	if fingerprint == "" || len(req.Questions) != 3 {
		t.Fatal("missing fingerprint or questions")
	}
	if _, ok := req.Questions["internal_instructions"].(NoulQuestion); !ok {
		t.Fatal("risk must be Noul")
	}
	if _, ok := req.Questions["intent"].(ChoiceQuestion); !ok {
		t.Fatal("intent must be Choice")
	}
}

func TestIntentRequiresBothRisksBeforeUsingChoice(t *testing.T) {
	for _, tc := range []struct {
		name             string
		internal, bypass Answer
		choice           Answer
		want             string
	}{
		{"both_risks", NoulAnswer{Noul: 1}, NoulAnswer{Noul: 1}, ChoiceAnswer{Choice: model.InputGreeting, Confidence: 1, Probabilities: map[string]float64{model.InputGreeting: 1}}, model.InputInternalInstructions},
		{"missing_risk", NoulAnswer{Noul: 1}, nil, nil, model.InputDetectionFailed},
		{"invalid_risk", NoulAnswer{Noul: math.NaN()}, NoulAnswer{Noul: 0}, nil, model.InputDetectionFailed},
		{"missing_choice", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, nil, model.InputRequest},
		{"uncertain_choice", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputGreeting, Confidence: .4, Probabilities: map[string]float64{model.InputGreeting: .9}}, model.InputRequest},
		{"greeting", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputGreeting, Confidence: .95, Probabilities: map[string]float64{model.InputGreeting: .95}}, model.InputGreeting},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider := intentProvider(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Answers: map[string]Answer{"internal_instructions": tc.internal, "rule_bypass": tc.bypass, "intent": tc.choice}}, nil
			})
			got := ClassifyIntent(context.Background(), Snapshot{Provider: provider}, Request{})
			if got.Disposition != tc.want || calls != 1 {
				t.Fatalf("got %s calls=%d", got.Disposition, calls)
			}
		})
	}
}
