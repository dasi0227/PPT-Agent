// Package decision evaluates typed questions independently of generation models.
package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
)

type Provider interface {
	Evaluate(context.Context, Request) (Response, error)
}
type Snapshot struct {
	Provider Provider
	Identity string
}
type Source interface{ DecisionSnapshot() Snapshot }
type Request struct {
	State     any                 `json:"state"`
	Questions map[string]Question `json:"questions"`
}
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Invalid map[string]string `json:"invalid,omitempty"`
	Usage   Usage             `json:"usage"`
}
type Question interface {
	questionType() string
	MarshalJSON() ([]byte, error)
}
type NoulQuestion struct {
	Instructions any
	Criteria     map[string]any
}
type ChoiceQuestion struct {
	Instructions any
	Criteria     map[string]any
}
type ScoreQuestion struct {
	Instructions any
	Criteria     []any
}

func (NoulQuestion) questionType() string   { return "noul" }
func (ChoiceQuestion) questionType() string { return "choice" }
func (ScoreQuestion) questionType() string  { return "score" }
func questionJSON(kind string, instructions, criteria any) ([]byte, error) {
	return json.Marshal(struct {
		Type         string `json:"type"`
		Instructions any    `json:"instructions"`
		Criteria     any    `json:"criteria,omitempty"`
	}{kind, instructions, criteria})
}
func (q NoulQuestion) MarshalJSON() ([]byte, error) {
	if len(q.Criteria) == 0 {
		return questionJSON("noul", q.Instructions, nil)
	}
	return questionJSON("noul", q.Instructions, q.Criteria)
}
func (q ChoiceQuestion) MarshalJSON() ([]byte, error) {
	return questionJSON("choice", q.Instructions, q.Criteria)
}
func (q ScoreQuestion) MarshalJSON() ([]byte, error) {
	return questionJSON("score", q.Instructions, q.Criteria)
}

type Answer interface{ answerType() string }
type NoulAnswer struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}
type ChoiceAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}
type ScoreAnswer struct {
	Type          string             `json:"type"`
	Score         float64            `json:"score"`
	Legend        map[string]any     `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

func (NoulAnswer) answerType() string   { return "noul" }
func (ChoiceAnswer) answerType() string { return "choice" }
func (ScoreAnswer) answerType() string  { return "score" }
func (r Response) Noul(id string) (NoulAnswer, error) {
	a, ok := r.Answers[id].(NoulAnswer)
	if !ok {
		return a, errors.New("missing or invalid noul answer")
	}
	return a, nil
}
func (r Response) Choice(id string) (ChoiceAnswer, error) {
	a, ok := r.Answers[id].(ChoiceAnswer)
	if !ok {
		return a, errors.New("missing or invalid choice answer")
	}
	return a, nil
}
func (r Response) Score(id string) (ScoreAnswer, error) {
	a, ok := r.Answers[id].(ScoreAnswer)
	if !ok {
		return a, errors.New("missing or invalid score answer")
	}
	return a, nil
}

func structured(v any, allowNull bool) bool {
	b, err := json.Marshal(v)
	if err != nil {
		return false
	}
	if string(b) == "null" {
		return allowNull
	}
	return len(b) > 0 && (b[0] == '"' || b[0] == '{' || b[0] == '[')
}
func Validate(req Request) error {
	if !structured(req.State, false) || len(req.Questions) == 0 {
		return errors.New("state and questions are required")
	}
	for id, q := range req.Questions {
		if id == "" || q == nil {
			return errors.New("question id and question are required")
		}
		switch v := q.(type) {
		case NoulQuestion:
			if !structured(v.Instructions, false) {
				return errors.New("invalid noul instructions")
			}
			for k, c := range v.Criteria {
				if (k != "true" && k != "false") || !structured(c, false) {
					return errors.New("invalid noul criteria")
				}
			}
		case ChoiceQuestion:
			if !structured(v.Instructions, false) || len(v.Criteria) < 2 || len(v.Criteria) > 255 {
				return errors.New("invalid choice question")
			}
			for k, c := range v.Criteria {
				if k == "" || !structured(c, true) {
					return errors.New("invalid choice criteria")
				}
			}
		case ScoreQuestion:
			if !structured(v.Instructions, false) || len(v.Criteria) < 2 || len(v.Criteria) > 10 {
				return errors.New("invalid score question")
			}
			for _, c := range v.Criteria {
				if !structured(c, false) {
					return errors.New("invalid score criteria")
				}
			}
		default:
			return errors.New("unsupported question")
		}
	}
	return nil
}
func probability(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
func distribution(p map[string]float64, keys map[string]bool) bool {
	if len(p) != len(keys) {
		return false
	}
	sum := 0.0
	for k, v := range p {
		if !keys[k] || !probability(v) {
			return false
		}
		sum += v
	}
	return math.Abs(sum-1) < 0.015
}
func decodeAnswer(raw json.RawMessage, q Question) (Answer, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, errors.New("invalid answer JSON")
	}
	required := []string{"type"}
	switch q.questionType() {
	case "noul":
		required = append(required, "noul")
	case "choice":
		required = append(required, "choice", "probabilities", "confidence")
	case "score":
		required = append(required, "score", "legend", "probabilities", "confidence")
	}
	for _, k := range required {
		if len(fields[k]) == 0 || string(fields[k]) == "null" {
			return nil, fmt.Errorf("missing %s", k)
		}
	}
	if q.questionType() != "noul" {
		var values map[string]*float64
		if json.Unmarshal(fields["probabilities"], &values) != nil || len(values) == 0 {
			return nil, errors.New("invalid probabilities")
		}
		for _, v := range values {
			if v == nil {
				return nil, errors.New("missing probability")
			}
		}
	}
	var kind string
	_ = json.Unmarshal(fields["type"], &kind)
	if kind != q.questionType() {
		return nil, errors.New("answer type mismatch")
	}
	switch v := q.(type) {
	case NoulQuestion:
		var a NoulAnswer
		if json.Unmarshal(raw, &a) != nil || !probability(a.Noul) {
			return nil, errors.New("invalid noul")
		}
		return a, nil
	case ChoiceQuestion:
		var a ChoiceAnswer
		keys := map[string]bool{}
		for k := range v.Criteria {
			keys[k] = true
		}
		if json.Unmarshal(raw, &a) != nil || !keys[a.Choice] || !probability(a.Confidence) || !distribution(a.Probabilities, keys) {
			return nil, errors.New("invalid choice distribution")
		}
		for _, p := range a.Probabilities {
			if p > a.Probabilities[a.Choice]+0.001 {
				return nil, errors.New("choice is not maximum probability")
			}
		}
		return a, nil
	case ScoreQuestion:
		var a ScoreAnswer
		keys := map[string]bool{}
		for i := range v.Criteria {
			keys[strconv.Itoa(i)] = true
		}
		if json.Unmarshal(raw, &a) != nil || a.Score < 0 || a.Score > float64(len(keys)-1) || !probability(a.Confidence) || !distribution(a.Probabilities, keys) || len(a.Legend) != len(keys) {
			return nil, errors.New("invalid score distribution")
		}
		expected := 0.0
		for i := range v.Criteria {
			k := strconv.Itoa(i)
			if _, ok := a.Legend[k]; !ok {
				return nil, errors.New("invalid score legend")
			}
			expected += float64(i) * a.Probabilities[k]
		}
		if math.Abs(expected-a.Score) > 0.05 {
			return nil, errors.New("score does not match distribution")
		}
		return a, nil
	}
	return nil, errors.New("unsupported answer")
}
