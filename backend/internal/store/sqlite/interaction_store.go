package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

type interactionPO struct {
	ID              string `gorm:"primaryKey"`
	RunID           string
	ThreadID        string
	Kind            string
	CallID          string
	RequestJSON     string
	RequestRevision int64
	State           string
	AnswerJSON      *string
	AnswerHash      *string
	CandidateJSON   *string
	AnsweredSeq     *int64
	AppliedSeq      *int64
	EntityRevision  int64
	UpdatedSeq      int64
	CreatedAt       int64
	UpdatedAt       int64
}

func (interactionPO) TableName() string { return "run_interactions" }
func (p interactionPO) model() model.Interaction {
	return model.Interaction{ID: p.ID, RunID: p.RunID, ThreadID: p.ThreadID, Kind: p.Kind, CallID: p.CallID,
		RequestRevision: p.RequestRevision, State: p.State, Request: json.RawMessage(p.RequestJSON), Answer: rawOrNull(p.AnswerJSON), Candidate: rawOrNull(p.CandidateJSON)}
}

func answerDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// Reads current entities in one short snapshot; no log or event reference is
// involved in displaying, resuming, or answering an interaction.
func (s *Store) Interaction(ctx context.Context, runID, kind, id string) (model.Interaction, error) {
	var result model.Interaction
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row interactionPO
		if err := tx.Where("id = ? AND run_id = ? AND kind = ?", id, runID, kind).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		head, err := readSessionHead(tx, row.ThreadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		result = row.model()
		return nil
	})
	return result, err
}

func (s *Store) AcceptInteractionAnswer(ctx context.Context, runID, kind, id string, answer json.RawMessage) (sessionstate.Receipt, error) {
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return sessionstate.Receipt{}, sessionstate.ErrSceneChanged
	}
	expectedRevision, ok := sessionstate.InteractionRevision(ctx)
	if !ok {
		return sessionstate.Receipt{}, sessionstate.ErrVersionConflict
	}
	current, err := s.Interaction(ctx, runID, kind, id)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	if current.RequestRevision != expectedRevision {
		return sessionstate.Receipt{}, sessionstate.ErrVersionConflict
	}
	normalized, err := model.NormalizeInteractionAnswer(current, answer)
	if err != nil {
		return sessionstate.Receipt{}, run.ErrReplyMismatch
	}
	digest := answerDigest(normalized)
	var owner runPO
	if err := s.reader.WithContext(ctx).Where("id = ?", runID).Take(&owner).Error; err != nil {
		return sessionstate.Receipt{}, mapErr(err)
	}
	return s.commitSession(ctx, sessionstate.Fence{ProjectID: owner.ProjectID, ThreadID: owner.ThreadID, SceneRevision: scene},
		sessionstate.CommitIdentity{Key: "interaction-answer:" + runID + ":" + id, RequestHash: fmt.Sprintf("%d:%s", expectedRevision, digest)}, func(w *sessionWrite) error {
			var row interactionPO
			if err := w.tx.Where("id = ? AND run_id = ? AND kind = ? AND thread_id = ?", id, runID, kind, w.head.ID).Take(&row).Error; err != nil {
				return mapErr(err)
			}
			if row.State == "answered" || row.State == "applied" {
				if textOrEmpty(row.AnswerHash) == digest {
					return nil
				}
				return run.ErrReplyMismatch
			}
			if row.State != "pending" || row.RequestRevision != current.RequestRevision {
				return sessionstate.ErrVersionConflict
			}
			if err := w.tx.Where("id = ?", runID).Take(&owner).Error; err != nil {
				return mapErr(err)
			}
			if owner.SceneRevision != scene || model.RunStatus(owner.Status).Terminal() || owner.CancelRequestedAt != nil {
				return run.ErrRunNotRunning
			}
			validated, err := model.NormalizeInteractionAnswer(row.model(), normalized)
			if err != nil || answerDigest(validated) != digest {
				return run.ErrReplyMismatch
			}
			value := string(validated)
			row.State, row.AnswerJSON, row.AnswerHash = "answered", &value, &digest
			row.AnsweredSeq = &w.seq
			row.EntityRevision++
			row.UpdatedAt, row.UpdatedSeq = w.now, w.seq
			if err := w.tx.Save(&row).Error; err != nil {
				return err
			}
			return w.projectInteraction(row)
		})
}

// Request creation belongs to the same transaction as the waiting checkpoint.
func (w *sessionWrite) checkpointInteractions(cp workflow.RuntimeCheckpoint) error {
	if pending := cp.PendingQuestion; pending != nil {
		if err := w.putInteraction(cp.RunID, "question", pending.Question.QuestionID, pending.Call.ID, 1, model.EventQuestionAsked, pending.Question, nil); err != nil {
			return err
		}
	}
	if pending := cp.PendingPlanCall; pending != nil {
		if err := w.putInteraction(cp.RunID, "plan", pending.Request.InteractionID, pending.Call.ID, 1, model.EventPlanApprovalRequested, pending.Request, nil); err != nil {
			return err
		}
	}
	if pending := cp.PendingCommand; pending != nil {
		if err := w.putInteraction(cp.RunID, "command", pending.InteractionID, pending.CallID, 1, model.EventCommandPermissionRequested, pending.Request, nil); err != nil {
			return err
		}
	}
	if pending := cp.PendingResourceApproval; pending != nil {
		if err := w.putInteraction(cp.RunID, "resource", pending.InteractionID, pending.CallID, pending.Request.Revision, model.EventResourceEditApprovalRequested, pending.Request, pending.Candidate); err != nil {
			return err
		}
	}
	if pending := cp.PendingScopeExpansion; pending != nil && !pending.Automatic {
		if err := w.putInteraction(cp.RunID, "scope", pending.Request.InteractionID, pending.Request.CallID, pending.Request.BaseRevision, model.EventScopeExpansionRequested, pending.Request, nil); err != nil {
			return err
		}
	}
	for _, application := range cp.InteractionApplications {
		var row interactionPO
		if err := w.tx.Where("id = ? AND run_id = ? AND thread_id = ?", application.ID, cp.RunID, w.head.ID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		answer, err := model.NormalizeInteractionAnswer(row.model(), application.Answer)
		if err != nil || row.AnswerHash == nil || answerDigest(answer) != *row.AnswerHash {
			return run.ErrReplyMismatch
		}
		if row.State == "applied" {
			continue
		}
		if row.State != "answered" {
			return sessionstate.ErrVersionConflict
		}
		row.State, row.AppliedSeq = "applied", &w.seq
		row.EntityRevision++
		row.UpdatedSeq, row.UpdatedAt = w.seq, w.now
		if err := w.tx.Save(&row).Error; err != nil {
			return err
		}
		if err := w.projectInteraction(row); err != nil {
			return err
		}
	}
	return nil
}

func interactionRequestIdentity(raw []byte) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return "", err
	}
	delete(fields, "occurred_at")
	normalized, err := json.Marshal(fields)
	return answerDigest(normalized), err
}

func (w *sessionWrite) putInteraction(runID, kind, id, callID string, revision int64, event model.EventType, request any, candidate *model.ResourceEditCandidate) error {
	if id == "" || callID == "" || revision < 1 {
		return sessionstate.ErrInvalidState
	}
	if err := model.ValidatePublicEvent(event, request); err != nil {
		return err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var identity struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil || identity.RunID != runID {
		return sessionstate.ErrInvalidState
	}
	if (kind == "resource") != (candidate != nil) {
		return sessionstate.ErrInvalidState
	}
	var candidateJSON *string
	if candidate != nil {
		if !json.Valid(candidate.Proposal) || !json.Valid(candidate.Draft) ||
			(candidate.BaseExists && !json.Valid(candidate.Base)) ||
			(!candidate.BaseExists && len(candidate.Base) != 0 && string(candidate.Base) != "null") {
			return sessionstate.ErrInvalidState
		}
		encoded, err := json.Marshal(candidate)
		if err != nil {
			return err
		}
		value := string(encoded)
		candidateJSON = &value
	}
	var row interactionPO
	err = w.tx.Where("id = ?", id).Take(&row).Error
	if err == nil {
		if row.ThreadID != w.head.ID || row.RunID != runID || row.Kind != kind || row.CallID != callID {
			return sessionstate.ErrInvalidState
		}
		// A resource draft may have advanced while the worker was waiting. An
		// older checkpoint reference must never overwrite that current request.
		if kind == "resource" && revision < row.RequestRevision {
			return nil
		}
		oldHash, hashErr := interactionRequestIdentity([]byte(row.RequestJSON))
		if hashErr != nil {
			return hashErr
		}
		newHash, hashErr := interactionRequestIdentity(raw)
		if hashErr != nil {
			return hashErr
		}
		if revision == row.RequestRevision && oldHash == newHash && textOrEmpty(row.CandidateJSON) == textOrEmpty(candidateJSON) {
			return nil
		}
		if kind != "resource" || row.State != "pending" || revision != row.RequestRevision+1 {
			return sessionstate.ErrVersionConflict
		}
		row.RequestJSON, row.RequestRevision = string(raw), revision
		row.CandidateJSON = candidateJSON
		row.EntityRevision++
		row.UpdatedSeq, row.UpdatedAt = w.seq, w.now
		if err := w.tx.Save(&row).Error; err != nil {
			return err
		}
	} else {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = interactionPO{ID: id, RunID: runID, ThreadID: w.head.ID, Kind: kind, CallID: callID, RequestJSON: string(raw), RequestRevision: revision, CandidateJSON: candidateJSON,
			State: "pending", EntityRevision: 1, UpdatedSeq: w.seq, CreatedAt: w.now, UpdatedAt: w.now}
		if err := w.tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return w.projectInteraction(row)
}

func (w *sessionWrite) projectInteraction(row interactionPO) error {
	view := sessionstate.InteractionView{ID: row.ID, ThreadID: row.ThreadID, RunID: row.RunID, CallID: row.CallID, Kind: row.Kind, State: row.State,
		RequestRevision: row.RequestRevision, EntityRevision: row.EntityRevision, UpdatedSeq: row.UpdatedSeq, Request: json.RawMessage(row.RequestJSON), Answer: rawOrNull(row.AnswerJSON)}
	raw, err := json.Marshal(view)
	if err != nil {
		return err
	}
	itemID := "interaction:" + row.ID
	if len(raw) > 32*1024 {
		detailID := model.MustShortID("detail")
		if err := w.tx.Table("timeline_details").Create(map[string]any{"id": detailID, "thread_id": w.head.ID, "public_json": string(raw), "created_seq": w.seq}).Error; err != nil {
			return err
		}
		var itemRevision int64
		if err := w.tx.Table("timeline_items").Select("COALESCE(MAX(entity_revision), 0)").Where("id = ? AND thread_id = ?", itemID, w.head.ID).Scan(&itemRevision).Error; err != nil {
			return err
		}
		view.DetailRef = sessionstate.ContentRef{ThreadID: w.head.ID, ItemID: itemID, SceneRevision: w.head.SceneRevision, ItemRevision: itemRevision + 1, SourceType: "detail", SourceID: detailID}.Encode()
		view.Request, view.Answer = nil, nil
		raw, err = json.Marshal(view)
		if err != nil {
			return err
		}
	}
	if err := w.putItem(sessionstate.TimelineItem{ID: itemID, ThreadID: w.head.ID, GroupID: "run:" + row.RunID, Kind: "interaction", InteractionID: row.ID, Summary: raw, DetailRef: view.DetailRef}); err != nil {
		return err
	}
	return w.upsert("interaction", row.ID, view)
}

// Run termination invalidates outstanding requests in the same commit. Accepted
// answers and candidate content remain available for diagnostics and retries.
func (w *sessionWrite) cancelRunInteractions(runID string) error {
	var rows []interactionPO
	if err := w.tx.Where("run_id = ? AND state IN ?", runID, []string{"pending", "answered"}).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		row.State = "canceled"
		row.EntityRevision++
		row.UpdatedSeq, row.UpdatedAt = w.seq, w.now
		if err := w.tx.Save(&row).Error; err != nil {
			return err
		}
		if err := w.projectInteraction(row); err != nil {
			return err
		}
	}
	return nil
}

func resourceDraftIdentity(runID, interactionID string, revision int64, draft json.RawMessage) (sessionstate.CommitIdentity, error) {
	if runID == "" || interactionID == "" || revision < 1 || !json.Valid(draft) {
		return sessionstate.CommitIdentity{}, sessionstate.ErrInvalidState
	}
	var body any
	decoder := json.NewDecoder(bytes.NewReader(draft))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return sessionstate.CommitIdentity{}, err
	}
	hash, err := commandHash(body)
	return sessionstate.CommitIdentity{Key: fmt.Sprintf("resource-draft:%s:%s:%d", runID, interactionID, revision), RequestHash: hash}, err
}

func (s *Store) ResourceDraftReceipt(ctx context.Context, runID, interactionID string, revision int64, draft json.RawMessage) (model.ResourceEditApproval, bool, error) {
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return model.ResourceEditApproval{}, false, sessionstate.ErrSceneChanged
	}
	identity, err := resourceDraftIdentity(runID, interactionID, revision, draft)
	if err != nil {
		return model.ResourceEditApproval{}, false, err
	}
	var result model.ResourceEditApproval
	found := false
	err = s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner struct{ ThreadID string }
		if err := tx.Table("runs").Select("thread_id").Where("id=?", runID).Take(&owner).Error; err != nil {
			return mapErr(err)
		}
		head, err := readSessionHead(tx, owner.ThreadID)
		if err != nil {
			return err
		}
		if head.SceneRevision != scene {
			return sessionstate.ErrSceneChanged
		}
		var stored idempotencyPO
		err = tx.Where("scope=? AND owner_id=? AND key=?", "session:"+strconv.FormatInt(scene, 10), owner.ThreadID, identity.Key).Take(&stored).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if stored.RequestHash != identity.RequestHash {
			return sessionstate.ErrIdempotencyConflict
		}
		if stored.Status != "completed" {
			return sessionstate.ErrInvalidState
		}
		var receipt sessionstate.Receipt
		if err := json.Unmarshal([]byte(stored.ResultJSON), &receipt); err != nil {
			return err
		}
		if err := json.Unmarshal(receipt.Result, &result); err != nil {
			return err
		}
		if result.RunID != runID || result.InteractionID != interactionID {
			return sessionstate.ErrInvalidState
		}
		found = true
		return nil
	})
	return result, found, err
}

func (s *Store) UpdateResourceInteraction(ctx context.Context, record model.ResourceEditApproval, draft json.RawMessage) error {
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return sessionstate.ErrSceneChanged
	}
	expectedRevision, ok := sessionstate.InteractionRevision(ctx)
	if !ok {
		return sessionstate.ErrVersionConflict
	}
	identity, err := resourceDraftIdentity(record.RunID, record.InteractionID, expectedRevision, draft)
	if err != nil {
		return err
	}
	request := model.ResourceEditApprovalRequestedPayload{PublicEventBase: model.NewPublicEventBase(record.RunID),
		InteractionID: record.InteractionID, CallID: record.CallID, Resource: record.Resource, Revision: record.Revision, Target: record.Target}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	var owner runPO
	if err := s.reader.WithContext(ctx).Where("id = ?", record.RunID).Take(&owner).Error; err != nil {
		return mapErr(err)
	}
	_, err = s.commitSession(ctx, sessionstate.Fence{ProjectID: owner.ProjectID, ThreadID: owner.ThreadID, SceneRevision: scene},
		identity, func(w *sessionWrite) error {
			var row runPO
			if err := w.tx.Where("id = ?", record.RunID).Take(&row).Error; err != nil {
				return mapErr(err)
			}
			if model.RunStatus(row.Status).Terminal() || row.CancelRequestedAt != nil {
				return run.ErrRunNotRunning
			}
			var existing interactionPO
			if err := w.tx.Where("id = ? AND run_id = ? AND kind = 'resource'", request.InteractionID, record.RunID).Take(&existing).Error; err != nil {
				return mapErr(err)
			}
			if existing.RequestRevision != expectedRevision || existing.State != "pending" {
				return sessionstate.ErrVersionConflict
			}
			if (record.Revision != expectedRevision+1 && record.Revision != expectedRevision) || record.State != "pending" || record.Answer != nil {
				return sessionstate.ErrVersionConflict
			}
			previous, err := existing.model().ResourceEditApproval()
			if err != nil {
				return err
			}
			if previous.Resource != record.Resource || previous.BaseExists != record.BaseExists || previous.BaseHash != record.BaseHash ||
				string(previous.Base) != string(record.Base) || string(previous.Proposal) != string(record.Proposal) {
				return sessionstate.ErrInvalidState
			}
			if record.Revision == expectedRevision {
				before, _ := json.Marshal(previous.ResourceEditCandidate)
				after, _ := json.Marshal(record.ResourceEditCandidate)
				if !bytes.Equal(before, after) {
					return sessionstate.ErrInvalidState
				}
			} else if err := w.putInteraction(record.RunID, "resource", request.InteractionID, request.CallID, request.Revision, model.EventResourceEditApprovalUpdated, request, &record.ResourceEditCandidate); err != nil {
				return err
			}
			w.result = raw
			return nil
		})
	return err
}
