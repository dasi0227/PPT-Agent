package sqlite

import (
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

// Persist control-flow references, not duplicate copies of mutable requests.
// Copies protect the Runtime candidate from storage-only normalization.
func checkpointReferences(cp workflow.RuntimeCheckpoint) workflow.RuntimeCheckpoint {
	if cp.PendingQuestion != nil {
		pending := *cp.PendingQuestion
		pending.Question = model.QuestionAskedPayload{QuestionID: pending.Question.QuestionID}
		cp.PendingQuestion = &pending
	}
	if cp.PendingPlanCall != nil {
		pending := *cp.PendingPlanCall
		pending.Request = model.PlanApprovalRequestedPayload{InteractionID: pending.Request.InteractionID}
		cp.PendingPlanCall = &pending
	}
	if cp.PendingCommand != nil {
		pending := *cp.PendingCommand
		pending.Request = model.CommandPermissionRequestedPayload{}
		pending.Answer = nil
		cp.PendingCommand = &pending
	}
	if cp.PendingResourceApproval != nil {
		pending := *cp.PendingResourceApproval
		pending.Request = model.ResourceEditApprovalRequestedPayload{}
		pending.Answer = nil
		pending.Candidate = nil
		cp.PendingResourceApproval = &pending
	}
	if cp.PendingScopeExpansion != nil && !cp.PendingScopeExpansion.Automatic {
		pending := *cp.PendingScopeExpansion
		pending.Request = model.ScopeExpansionRequestedPayload{InteractionID: pending.Request.InteractionID}
		cp.PendingScopeExpansion = &pending
	}
	return cp
}

func hydrateCheckpointInteractions(tx *gorm.DB, cp *workflow.RuntimeCheckpoint) error {
	type reference struct {
		id, kind string
		request  any
	}
	refs := []reference{}
	if pending := cp.PendingQuestion; pending != nil {
		refs = append(refs, reference{pending.Question.QuestionID, "question", &pending.Question})
	}
	if pending := cp.PendingPlanCall; pending != nil {
		refs = append(refs, reference{pending.Request.InteractionID, "plan", &pending.Request})
	}
	if pending := cp.PendingCommand; pending != nil {
		refs = append(refs, reference{pending.InteractionID, "command", &pending.Request})
	}
	if pending := cp.PendingResourceApproval; pending != nil {
		refs = append(refs, reference{pending.InteractionID, "resource", &pending.Request})
	}
	if pending := cp.PendingScopeExpansion; pending != nil && !pending.Automatic {
		refs = append(refs, reference{pending.Request.InteractionID, "scope", &pending.Request})
	}
	if len(refs) == 0 {
		return nil
	}
	ids := make([]string, len(refs))
	for i, ref := range refs {
		ids[i] = ref.id
	}
	var rows []interactionPO
	if err := tx.Where("run_id = ? AND id IN ?", cp.RunID, ids).Find(&rows).Error; err != nil {
		return err
	}
	byID := make(map[string]interactionPO, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	for _, ref := range refs {
		row, ok := byID[ref.id]
		if !ok || row.Kind != ref.kind {
			return fmt.Errorf("%w: missing checkpoint interaction %s", sessionstate.ErrInvalidState, ref.id)
		}
		if err := json.Unmarshal([]byte(row.RequestJSON), ref.request); err != nil {
			return err
		}
		if row.Kind == "resource" {
			record, err := row.model().ResourceEditApproval()
			if err != nil {
				return err
			}
			cp.PendingResourceApproval.Candidate = &record.ResourceEditCandidate
		}
		if row.Kind == "command" && row.State == "applied" {
			if row.AnswerJSON == nil {
				return sessionstate.ErrInvalidState
			}
			var answer model.CommandPermissionAnswer
			if err := json.Unmarshal([]byte(*row.AnswerJSON), &answer); err != nil {
				return err
			}
			cp.PendingCommand.Answer = &answer
		}
	}
	return nil
}
