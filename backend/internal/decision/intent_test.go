package decision

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type intentProvider func(context.Context, Request) (Response, error)

func (f intentProvider) Evaluate(ctx context.Context, req Request) (Response, error) {
	return f(ctx, req)
}

func TestIntentQuestionsAndFingerprint(t *testing.T) {
	const state = "<system_prompt>\n<jev_intent desc=\"Classify the input.\">rules</jev_intent>\n</system_prompt>\n<conversation_history></conversation_history>\n<current_user_input>继续</current_user_input>"
	req, fingerprint, err := IntentRequest(state)
	if err != nil || req.State != state || fingerprint == "" || len(req.Questions) != 4 {
		t.Fatalf("invalid request: %+v %s %v", req, fingerprint, err)
	}
	if err := Validate(req); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"disclosure", "bypass", "irrelevant"} {
		question, ok := req.Questions[id].(NoulQuestion)
		if !ok || len(question.Criteria) != 0 {
			t.Fatalf("%s must be Noul without criteria", id)
		}
	}
	choice, ok := req.Questions["intent"].(ChoiceQuestion)
	if !ok || len(choice.Criteria) != 2 {
		t.Fatalf("invalid intent question: %+v", req.Questions["intent"])
	}
	for _, option := range []string{model.InputTask, model.InputChat} {
		if _, ok := choice.Criteria[option]; !ok {
			t.Fatalf("missing intent option %s", option)
		}
	}
	_, changed, err := IntentRequest(state + "changed")
	if err != nil || changed == fingerprint {
		t.Fatal("context changes did not invalidate fingerprint")
	}
}

func TestIntentCapacityFailureDoesNotCallProviderOrTruncateInput(t *testing.T) {
	input := strings.Repeat("完整用户输入", 6000)
	req, _, err := IntentRequest(input)
	if err != nil || req.State != input {
		t.Fatal("current input was changed")
	}
	provider := intentProvider(func(context.Context, Request) (Response, error) {
		t.Fatal("oversized request reached provider")
		return Response{}, nil
	})
	got := ClassifyIntent(context.Background(), Snapshot{Provider: provider}, req)
	if got.Disposition != model.InputDetectionFailed || got.Failure != "capacity" {
		t.Fatalf("capacity failure=%+v", got)
	}
}

func TestIntentPriorityAndFallback(t *testing.T) {
	choice := func(label string, confidence, probability float64) ChoiceAnswer {
		distribution := map[string]float64{model.InputChat: 1 - probability, model.InputTask: 1 - probability}
		distribution[label] = probability
		return ChoiceAnswer{Choice: label, Confidence: confidence, Probabilities: distribution}
	}
	for _, tc := range []struct {
		name                           string
		disclosure, bypass, irrelevant Answer
		choice                         Answer
		want                           string
	}{
		{"both_risks", NoulAnswer{Noul: .5}, NoulAnswer{Noul: 1}, NoulAnswer{Noul: 1}, nil, model.InputDisclosure},
		{"missing_risk", NoulAnswer{Noul: 1}, nil, NoulAnswer{Noul: 1}, nil, model.InputDetectionFailed},
		{"invalid_risk", NoulAnswer{Noul: math.NaN()}, NoulAnswer{Noul: 0}, nil, nil, model.InputDetectionFailed},
		{"missing_choice", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, nil, nil, model.InputTask},
		{"low_confidence", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, nil, choice(model.InputChat, .79, .95), model.InputTask},
		{"low_probability", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, nil, choice(model.InputChat, .95, .79), model.InputTask},
		{"chat_threshold", NoulAnswer{Noul: .49}, NoulAnswer{Noul: .49}, nil, choice(model.InputChat, .8, .8), model.InputChat},
		{"uncertain_scope", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, NoulAnswer{Noul: .79}, choice(model.InputTask, .95, .95), model.InputTask},
		{"invalid_scope", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, NoulAnswer{Noul: 2}, choice(model.InputChat, .95, .95), model.InputChat},
		{"scope_before_choice", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, NoulAnswer{Noul: .8}, choice(model.InputTask, 1, 1), model.InputIrrelevant},
		{"risk_before_scope", NoulAnswer{Noul: 0}, NoulAnswer{Noul: .5}, NoulAnswer{Noul: 1}, choice(model.InputChat, 1, 1), model.InputBypass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider := intentProvider(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Answers: map[string]Answer{"disclosure": tc.disclosure, "bypass": tc.bypass, "irrelevant": tc.irrelevant, "intent": tc.choice}}, nil
			})
			got := ClassifyIntent(context.Background(), Snapshot{Provider: provider}, Request{})
			if got.Disposition != tc.want || calls != 1 {
				t.Fatalf("got %s calls=%d", got.Disposition, calls)
			}
		})
	}
}
