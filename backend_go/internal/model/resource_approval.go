package model

import (
	"encoding/json"
	"errors"
)

// ResourceEditCandidate is the private editor snapshot. Decision and request
// state live only on the enclosing interaction row.
type ResourceEditCandidate struct {
	BaseExists bool            `json:"base_exists"`
	BaseHash   string          `json:"base_hash"`
	Base       json.RawMessage `json:"base"`
	Proposal   json.RawMessage `json:"proposal"`
	Draft      json.RawMessage `json:"draft"`
}

// ResourceEditApproval is assembled for the explicit editor read and Runtime.
// It is never a second persisted copy of the interaction state.
type ResourceEditApproval struct {
	ResourceEditCandidate
	RunID         string                      `json:"run_id"`
	CallID        string                      `json:"call_id"`
	InteractionID string                      `json:"interaction_id"`
	Resource      string                      `json:"resource"`
	Revision      int64                       `json:"revision,string"`
	Target        PublicTarget                `json:"target"`
	State         string                      `json:"state"`
	Answer        *ResourceEditApprovalAnswer `json:"answer,omitempty"`
}

func (i Interaction) ResourceEditApproval() (ResourceEditApproval, error) {
	var request ResourceEditApprovalRequestedPayload
	var candidate ResourceEditCandidate
	if i.Kind != "resource" || len(i.Candidate) == 0 || string(i.Candidate) == "null" ||
		json.Unmarshal(i.Request, &request) != nil || json.Unmarshal(i.Candidate, &candidate) != nil ||
		request.InteractionID != i.ID || request.RunID != i.RunID || request.CallID != i.CallID || request.Revision != i.RequestRevision {
		return ResourceEditApproval{}, errors.New("invalid resource approval snapshot")
	}
	result := ResourceEditApproval{ResourceEditCandidate: candidate,
		RunID: i.RunID, CallID: i.CallID, InteractionID: i.ID, Resource: request.Resource,
		Revision: i.RequestRevision, Target: request.Target, State: i.State}
	if i.State == "answered" || i.State == "applied" {
		var answer ResourceEditApprovalAnswer
		normalized, err := NormalizeInteractionAnswer(i, i.Answer)
		if err != nil {
			return ResourceEditApproval{}, err
		}
		if err := json.Unmarshal(normalized, &answer); err != nil {
			return ResourceEditApproval{}, err
		}
		result.Answer = &answer
	}
	return result, nil
}
