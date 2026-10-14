package decision

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIClient adapts Decisions to the same typed boundary used by Jev.
// NewDecisionService avoids implicit environment credentials or model routing.
type OpenAIClient struct {
	service openai.DecisionService
	model   string
}

func (*OpenAIClient) String() string     { return "OpenAIDecisionClient{redacted}" }
func (c *OpenAIClient) GoString() string { return c.String() }
func NewPreviewSnapshot(cfg *config.PreviewConfig) (Snapshot, error) {
	if cfg == nil {
		return Snapshot{}, nil
	}
	value := *cfg
	if err := config.ValidatePreviewConfig(&value); err != nil {
		return Snapshot{}, err
	}
	identity := sha256.Sum256([]byte("openai-decisions\x00" + value.BaseURL + "\x00" + value.Model + "\x00" + value.Key))
	service := openai.NewDecisionService(option.WithAPIKey(value.Key), option.WithBaseURL(value.BaseURL+"/"), option.WithMaxRetries(0), option.WithHTTPClient(&http.Client{
		Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}))
	return Snapshot{Identity: hex.EncodeToString(identity[:]), Provider: &OpenAIClient{service: service, model: value.Model}}, nil
}
func decisionText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	raw, _ := json.Marshal(value)
	return string(raw)
}
func openAIRequest(req Request, model string) (openai.DecisionNewParams, error) {
	if err := Validate(req); err != nil {
		return openai.DecisionNewParams{}, &Error{Kind: "request"}
	}
	out := openai.DecisionNewParams{Model: model}
	if len(req.Input) == 0 {
		out.Input.OfString = openai.String(decisionText(req.State))
	} else {
		parts := make([]openai.DecisionInputPartUnionParam, 0, len(req.Input))
		for _, part := range req.Input {
			if part.Image != nil {
				parts = append(parts, openai.DecisionInputPartUnionParam{OfInputImage: &openai.DecisionInputImageParam{ImageURL: "data:" + part.Image.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(part.Image.Data)}})
			} else {
				parts = append(parts, openai.DecisionInputPartUnionParam{OfInputText: &openai.DecisionInputTextParam{Text: part.Text}})
			}
		}
		out.Input.OfDecisionInputMessageArray = []openai.DecisionInputMessageParam{{Content: openai.DecisionInputMessageContentUnionParam{OfParts: parts}}}
	}
	ids := make([]string, 0, len(req.Questions))
	for id := range req.Questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		switch q := req.Questions[id].(type) {
		case NoulQuestion:
			instructions := decisionText(q.Instructions)
			if len(q.Criteria) > 0 {
				instructions += "\nPredicate criteria: " + decisionText(q.Criteria)
			}
			out.Questions = append(out.Questions, openai.DecisionNewParamsQuestionUnion{OfPredicate: &openai.DecisionNewParamsQuestionPredicate{Name: openai.String(id), Instructions: instructions}})
		case ChoiceQuestion:
			keys := make([]string, 0, len(q.Criteria))
			for key := range q.Criteria {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			choices := make([]openai.DecisionNewParamsQuestionChoiceChoice, 0, len(keys))
			for _, key := range keys {
				choice := openai.DecisionNewParamsQuestionChoiceChoice{Value: openai.DecisionNewParamsQuestionChoiceChoiceValueUnion{OfString: openai.String(key)}}
				if q.Criteria[key] != nil {
					choice.Description = openai.String(decisionText(q.Criteria[key]))
				}
				choices = append(choices, choice)
			}
			out.Questions = append(out.Questions, openai.DecisionNewParamsQuestionUnion{OfChoice: &openai.DecisionNewParamsQuestionChoice{Name: openai.String(id), Instructions: decisionText(q.Instructions), Choices: choices}})
		case ScoreQuestion:
			levels := make([]openai.DecisionNewParamsQuestionScoreLevel, len(q.Criteria))
			for i, criterion := range q.Criteria {
				levels[i] = openai.DecisionNewParamsQuestionScoreLevel{Label: strconv.Itoa(i), Description: openai.String(decisionText(criterion))}
			}
			out.Questions = append(out.Questions, openai.DecisionNewParamsQuestionUnion{OfScore: &openai.DecisionNewParamsQuestionScore{Name: openai.String(id), Instructions: decisionText(q.Instructions), Levels: levels}})
		}
	}
	return out, nil
}
func (c *OpenAIClient) Evaluate(ctx context.Context, req Request) (Response, error) {
	body, err := openAIRequest(req, c.model)
	if err != nil {
		return Response{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	wire, err := c.service.New(callCtx, body)
	if err != nil {
		if callCtx.Err() != nil {
			return Response{}, callCtx.Err()
		}
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			kind := "http"
			if apiErr.StatusCode == 401 || apiErr.StatusCode == 403 {
				kind = "authentication"
			}
			return Response{}, &Error{Kind: kind, Status: apiErr.StatusCode}
		}
		return Response{}, &Error{Kind: "transport"}
	}
	if wire == nil || !wire.JSON.Answers.Valid() || !wire.JSON.Model.Valid() || wire.Model == "" || !wire.JSON.Usage.Valid() || !wire.Usage.JSON.InputTokens.Valid() || !wire.Usage.JSON.OutputTokens.Valid() || wire.Usage.InputTokens < 0 || wire.Usage.OutputTokens < 0 {
		return Response{}, &Error{Kind: "protocol"}
	}
	out := Response{Model: wire.Model, Usage: Usage{InputTokens: int(wire.Usage.InputTokens), OutputTokens: int(wire.Usage.OutputTokens)}, Answers: map[string]Answer{}, Invalid: map[string]string{}}
	seen := map[string]bool{}
	for _, answer := range wire.Answers {
		q, exists := req.Questions[answer.Name]
		if !exists || seen[answer.Name] {
			return Response{}, &Error{Kind: "protocol"}
		}
		seen[answer.Name] = true
		if answer.Type == "refusal" {
			out.Invalid[answer.Name] = "refused"
			continue
		}
		decoded, err := decodeOpenAIAnswer([]byte(answer.RawJSON()), q)
		if err != nil {
			out.Invalid[answer.Name] = "invalid_answer"
		} else {
			out.Answers[answer.Name] = decoded
		}
	}
	for id := range req.Questions {
		if !seen[id] {
			out.Invalid[id] = "missing_answer"
		}
	}
	return out, nil
}

// Validate presence before normalizing into the common answer validator. Zero
// scores and probabilities are valid values, never substitutes for missing data.
func decodeOpenAIAnswer(raw []byte, question Question) (Answer, error) {
	var wire struct {
		Type          string   `json:"type"`
		Choice        *string  `json:"choice"`
		Probability   *float64 `json:"probability"`
		Score         *float64 `json:"score"`
		Confidence    *float64 `json:"confidence"`
		Probabilities []struct {
			Value       json.RawMessage `json:"value"`
			Label       *string         `json:"label"`
			Probability *float64        `json:"probability"`
		} `json:"probabilities"`
	}
	if json.Unmarshal(raw, &wire) != nil {
		return nil, &Error{Kind: "protocol"}
	}
	normalized := map[string]any{"type": question.questionType()}
	switch q := question.(type) {
	case NoulQuestion:
		if wire.Type != "predicate" || wire.Probability == nil {
			return nil, &Error{Kind: "protocol"}
		}
		normalized["noul"] = *wire.Probability
	case ChoiceQuestion:
		if wire.Type != "choice" || wire.Choice == nil || wire.Confidence == nil {
			return nil, &Error{Kind: "protocol"}
		}
		probabilities := map[string]float64{}
		for _, item := range wire.Probabilities {
			var key *string
			if json.Unmarshal(item.Value, &key) != nil || key == nil || item.Probability == nil {
				return nil, &Error{Kind: "protocol"}
			}
			if _, duplicate := probabilities[*key]; duplicate {
				return nil, &Error{Kind: "protocol"}
			}
			probabilities[*key] = *item.Probability
		}
		normalized["choice"], normalized["confidence"], normalized["probabilities"] = *wire.Choice, *wire.Confidence, probabilities
	case ScoreQuestion:
		if wire.Type != "score" || wire.Score == nil || wire.Confidence == nil {
			return nil, &Error{Kind: "protocol"}
		}
		probabilities, legend := map[string]float64{}, map[string]any{}
		for i, criterion := range q.Criteria {
			legend[strconv.Itoa(i)] = criterion
		}
		for _, item := range wire.Probabilities {
			var value *int
			if json.Unmarshal(item.Value, &value) != nil || value == nil || item.Label == nil || item.Probability == nil || *value < 0 || *value >= len(q.Criteria) || *item.Label != strconv.Itoa(*value) {
				return nil, &Error{Kind: "protocol"}
			}
			key := strconv.Itoa(*value)
			if _, duplicate := probabilities[key]; duplicate {
				return nil, &Error{Kind: "protocol"}
			}
			probabilities[key] = *item.Probability
		}
		normalized["score"], normalized["confidence"], normalized["probabilities"], normalized["legend"] = *wire.Score, *wire.Confidence, probabilities, legend
	default:
		return nil, &Error{Kind: "request"}
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return decodeAnswer(encoded, question)
}
