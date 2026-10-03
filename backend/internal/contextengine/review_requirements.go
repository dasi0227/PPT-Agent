package contextengine

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

type ReviewUserInput struct {
	RunID         string                      `json:"run_id,omitempty"`
	Text          string                      `json:"text"`
	Attachments   []model.AttachmentReference `json:"attachments,omitempty"`
	DOMSelections []model.DOMSelection        `json:"dom_selections,omitempty"`
}

// Read authoritative user events, not the compacted model projection. Every
// requirement and correction survives model.edit removals, without including
// assistant text, reasoning, command logs or arbitrary tool output.
func (s *JournalTranscriptStore) LoadReviewUserInputs(ctx context.Context, workDir, threadID string) ([]ReviewUserInput, error) {
	var events []threadjournal.Event
	var err error
	if s.backend != nil {
		events, err = s.backend.ThreadEvents(ctx, threadID, 0)
	} else {
		events, err = s.events(workDir, threadID)
	}
	if err != nil {
		return nil, err
	}
	inputs := []ReviewUserInput{}
	questions := map[string][]model.QuestionField{}
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		input := ReviewUserInput{RunID: event.RunID}
		switch event.Type {
		case string(model.EventQuestionAsked):
			var value model.QuestionAskedPayload
			if err := json.Unmarshal(event.Payload, &value); err != nil {
				return nil, err
			}
			questions[value.QuestionID] = value.Questions
			continue
		case "run.accepted":
			var value struct {
				Command model.RunCommand `json:"command"`
			}
			if err := json.Unmarshal(event.Payload, &value); err != nil {
				return nil, err
			}
			input.Text, input.Attachments, input.DOMSelections = value.Command.Instruction, value.Command.Attachments, value.Command.DOMSelections
		case "steering.accepted":
			var value model.SteeringMessage
			if err := json.Unmarshal(event.Payload, &value); err != nil {
				return nil, err
			}
			input.Text, input.Attachments, input.DOMSelections = value.Content, value.Attachments, value.DOMSelections
		case string(model.EventQuestionAnswered):
			var value model.QuestionAnsweredPayload
			if err := json.Unmarshal(event.Payload, &value); err != nil {
				return nil, err
			}
			question, ok := questions[value.QuestionID]
			if !ok {
				return nil, threadjournal.ErrCorrupt
			}
			answer, err := json.Marshal(struct {
				Questions []model.QuestionField `json:"questions"`
				Answer    model.QuestionAnswer  `json:"answer"`
			}{question, value.Answer})
			if err != nil {
				return nil, err
			}
			input.Text = "User answers: " + string(answer)
		default:
			continue
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}
