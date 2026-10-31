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
	if err != nil || req.State != state || fingerprint == "" || len(req.Questions) != 3 {
		t.Fatalf("invalid request: %+v %s %v", req, fingerprint, err)
	}
	if err := Validate(req); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"disclosure", "bypass"} {
		question, ok := req.Questions[id].(NoulQuestion)
		if !ok || len(question.Criteria) != 0 {
			t.Fatalf("%s must be Noul without criteria", id)
		}
	}
	choice, ok := req.Questions["intent"].(ChoiceQuestion)
	if !ok || len(choice.Criteria) != 4 {
		t.Fatalf("invalid intent question: %+v", req.Questions["intent"])
	}
	for _, option := range []string{"greeting", "identity", "ppt", "irrelevant"} {
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

func TestIntentRequiresBothRisksBeforeUsingChoice(t *testing.T) {
	for _, tc := range []struct {
		name               string
		disclosure, bypass Answer
		choice             Answer
		want               string
	}{
		{"both_risks", NoulAnswer{Noul: 1}, NoulAnswer{Noul: 1}, ChoiceAnswer{Choice: model.InputGreeting, Confidence: 1, Probabilities: map[string]float64{model.InputGreeting: 1}}, model.InputDisclosure},
		{"missing_risk", NoulAnswer{Noul: 1}, nil, nil, model.InputDetectionFailed},
		{"invalid_risk", NoulAnswer{Noul: math.NaN()}, NoulAnswer{Noul: 0}, nil, model.InputDetectionFailed},
		{"missing_choice", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, nil, model.InputPPT},
		{"uncertain_choice", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputGreeting, Confidence: .4, Probabilities: map[string]float64{model.InputGreeting: .9}}, model.InputPPT},
		{"greeting", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputGreeting, Confidence: .95, Probabilities: map[string]float64{model.InputGreeting: .95}}, model.InputGreeting},
		{"identity", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputIdentity, Confidence: .95, Probabilities: map[string]float64{model.InputIdentity: .95}}, model.InputIdentity},
		{"ppt", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputPPT, Confidence: .95, Probabilities: map[string]float64{model.InputPPT: .95}}, model.InputPPT},
		{"irrelevant", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputIrrelevant, Confidence: .95, Probabilities: map[string]float64{model.InputIrrelevant: .95}}, model.InputIrrelevant},
		{"uncertain_irrelevant", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputIrrelevant, Confidence: .4, Probabilities: map[string]float64{model.InputIrrelevant: .95}}, model.InputPPT},
		{"unlikely_irrelevant", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 0}, ChoiceAnswer{Choice: model.InputIrrelevant, Confidence: .95, Probabilities: map[string]float64{model.InputIrrelevant: .6}}, model.InputPPT},
		{"risk_before_irrelevant", NoulAnswer{Noul: 0}, NoulAnswer{Noul: 1}, ChoiceAnswer{Choice: model.InputIrrelevant, Confidence: .95, Probabilities: map[string]float64{model.InputIrrelevant: .95}}, model.InputBypass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider := intentProvider(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Answers: map[string]Answer{"disclosure": tc.disclosure, "bypass": tc.bypass, "intent": tc.choice}}, nil
			})
			got := ClassifyIntent(context.Background(), Snapshot{Provider: provider}, Request{})
			if got.Disposition != tc.want || calls != 1 {
				t.Fatalf("got %s calls=%d", got.Disposition, calls)
			}
		})
	}
}
