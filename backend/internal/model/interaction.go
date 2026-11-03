package model

import (
	"encoding/json"
	"errors"
	"strings"
)

// Interaction is the current durable request and answer, not an event history.
// Public endpoints use a separate bounded DTO and never return this record.
type Interaction struct {
	ID              string
	RunID           string
	ThreadID        string
	Kind            string
	CallID          string
	RequestRevision int64
	State           string
	Request         json.RawMessage
	Answer          json.RawMessage
	Candidate       json.RawMessage
}

// InteractionApplication accompanies the checkpoint that consumes an answer.
// The store compares the complete normalized answer to the accepted value.
type InteractionApplication struct {
	ID     string
	Answer json.RawMessage
}

type QuestionReply struct {
	QuestionID string         `json:"question_id"`
	Answer     QuestionAnswer `json:"answer"`
}

func ValidateQuestionAnswers(question QuestionAskedPayload, answer QuestionAnswer) (QuestionAnswer, bool) {
	if len(answer.Answers) != len(question.Questions) {
		return QuestionAnswer{}, false
	}
	byID := map[string]QuestionField{}
	for _, field := range question.Questions {
		byID[field.ID] = field
	}
	seen := map[string]bool{}
	for index, reply := range answer.Answers {
		field, ok := byID[reply.QuestionID]
		if !ok || seen[reply.QuestionID] {
			return QuestionAnswer{}, false
		}
		seen[reply.QuestionID] = true
		selected, custom := strings.TrimSpace(reply.SelectedOptionID), strings.TrimSpace(reply.CustomText)
		answer.Answers[index].SelectedOptionID = selected
		validOption := false
		for _, option := range field.Options {
			validOption = validOption || option.ID == selected
		}
		switch {
		case reply.Skipped:
			if selected != "" || custom != "" {
				return QuestionAnswer{}, false
			}
		case selected != "":
			if !validOption || custom != "" {
				return QuestionAnswer{}, false
			}
		case custom == "":
			return QuestionAnswer{}, false
		}
	}
	return answer, len(seen) == len(question.Questions)
}

// NormalizeInteractionAnswer validates against the persisted request, including
// its resource/scope revision. It also defines semantic retry identity.
func NormalizeInteractionAnswer(current Interaction, raw json.RawMessage) (json.RawMessage, error) {
	invalid := errors.New("interaction answer does not match the current request")
	switch current.Kind {
	case "question":
		var request QuestionAskedPayload
		var reply QuestionReply
		if json.Unmarshal(current.Request, &request) != nil || json.Unmarshal(raw, &reply) != nil || reply.QuestionID != current.ID {
			return nil, invalid
		}
		answer, ok := ValidateQuestionAnswers(request, reply.Answer)
		if !ok {
			return nil, invalid
		}
		reply.Answer = answer
		return json.Marshal(reply)
	case "plan":
		var answer PlanApprovalAnswer
		if json.Unmarshal(raw, &answer) != nil || answer.InteractionID != current.ID || (answer.Decision != "approve" && answer.Decision != "refuse") {
			return nil, invalid
		}
		answer.IdempotencyKey = ""
		return json.Marshal(answer)
	case "command":
		var request CommandPermissionRequestedPayload
		var answer CommandPermissionAnswer
		if json.Unmarshal(current.Request, &request) != nil || json.Unmarshal(raw, &answer) != nil || answer.InteractionID != current.ID || answer.CallID != request.CallID || answer.CommandHash != request.CommandHash || (answer.Decision != "allow_once" && answer.Decision != "deny") {
			return nil, invalid
		}
		return json.Marshal(answer)
	case "scope":
		var request ScopeExpansionRequestedPayload
		var answer ScopeExpansionAnswer
		if json.Unmarshal(current.Request, &request) != nil || json.Unmarshal(raw, &answer) != nil || answer.InteractionID != current.ID || answer.CallID != request.CallID || answer.BaseRevision != request.BaseRevision || (answer.Decision != "approve" && answer.Decision != "revise" && answer.Decision != "refuse") {
			return nil, invalid
		}
		return json.Marshal(answer)
	case "resource":
		var request ResourceEditApprovalRequestedPayload
		var answer ResourceEditApprovalAnswer
		if json.Unmarshal(current.Request, &request) != nil || json.Unmarshal(raw, &answer) != nil || answer.InteractionID != current.ID || answer.CallID != request.CallID || answer.Revision != request.Revision || (answer.Decision != "approve" && answer.Decision != "reject") {
			return nil, invalid
		}
		return json.Marshal(answer)
	default:
		return nil, invalid
	}
}
