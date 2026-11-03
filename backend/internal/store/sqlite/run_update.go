package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func runUpdateIdentity(runID string, execution int64, update sessionstate.RunUpdate) (sessionstate.CommitIdentity, error) {
	var value struct {
		RunID     string               `json:"run_id"`
		MessageID string               `json:"message_id"`
		CallID    string               `json:"call_id"`
		Call      model.ModelCallUsage `json:"call"`
	}
	if update.ID == "" || json.Unmarshal(update.Payload, &value) != nil || value.RunID != runID {
		return sessionstate.CommitIdentity{}, sessionstate.ErrInvalidState
	}
	hash, err := interactionRequestIdentity(update.Payload)
	if err != nil {
		return sessionstate.CommitIdentity{}, err
	}
	key := update.ID
	switch update.Type {
	case model.EventMessageThinking, model.EventMessageMilestone, model.EventMessageFinal:
		key = value.MessageID
	case model.EventToolStarted, model.EventToolCompleted:
		key = value.CallID
	case model.EventRunStarted, model.EventRunResumed, model.EventRunCompleted, model.EventRunFailed, model.EventRunCanceled, model.EventRunError:
		key = "state"
	case model.EventRunUsageUpdated:
		key = value.Call.CallID
		execution = 0
	}
	if key == "" {
		return sessionstate.CommitIdentity{}, sessionstate.ErrInvalidState
	}
	return sessionstate.CommitIdentity{Key: fmt.Sprintf("run-update:%s:%d:%s:%s", runID, execution, update.Type, key), RequestHash: hash}, nil
}

// ApplyRunUpdate handles independent facts such as progress and usage. Runtime
// observations are passed to the same projector inside CommitRuntimeState.
func (s *Store) ApplyRunUpdate(ctx context.Context, runID string, update sessionstate.RunUpdate) (sessionstate.Receipt, error) {
	if update.Type == model.EventContextCompacted {
		return sessionstate.Receipt{}, sessionstate.ErrInvalidState
	}
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return sessionstate.Receipt{}, sessionstate.ErrSceneChanged
	}
	if err := model.ValidatePublicEvent(update.Type, update.Payload); err != nil {
		return sessionstate.Receipt{}, err
	}
	var current runPO
	if err := s.reader.WithContext(ctx).Where("id = ?", runID).Take(&current).Error; err != nil {
		return sessionstate.Receipt{}, mapErr(err)
	}
	owner, execution, hasLease := workflow.CheckpointOwnership(ctx)
	if !hasLease && update.Type != model.EventRunUsageUpdated {
		return sessionstate.Receipt{}, run.ErrRunRevisionConflict
	}
	identity, err := runUpdateIdentity(runID, execution, update)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	return s.commitSession(ctx, sessionstate.Fence{ProjectID: current.ProjectID, ThreadID: current.ThreadID, SceneRevision: scene}, identity, func(w *sessionWrite) error {
		var row runPO
		if err := w.tx.Where("id = ? AND thread_id = ?", runID, w.head.ID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		if row.SceneRevision != scene {
			return sessionstate.ErrSceneChanged
		}
		if update.Type != model.EventRunUsageUpdated && (row.ExecutionRevision != execution || (row.OwnerInstanceID != owner && !(model.RunStatus(row.Status).Terminal() && row.OwnerInstanceID == ""))) {
			return run.ErrRunRevisionConflict
		}
		if err := w.applyRunUpdate(row, update); err != nil {
			return err
		}
		return w.publishRun(runID)
	})
}

func (w *sessionWrite) applyRunUpdate(row runPO, update sessionstate.RunUpdate) error {
	if err := model.ValidatePublicEvent(update.Type, update.Payload); err != nil {
		return err
	}
	var base model.PublicEventBase
	if json.Unmarshal(update.Payload, &base) != nil || base.RunID != row.ID {
		return sessionstate.ErrInvalidState
	}
	terminal := model.RunStatus(row.Status).Terminal()
	if terminal && update.Type != model.EventRunUsageUpdated && update.Type != model.EventContextWindowUpdated && !update.Type.Terminal() {
		return run.ErrRunNotRunning
	}
	set := func(column string, value any) error {
		return w.tx.Table("runs").Where("id = ?", row.ID).Update(column, value).Error
	}
	raw := string(update.Payload)
	switch update.Type {
	case model.EventRunStarted:
		// Acceptance already saved the command, original input, and timeline group.
		// This notification never inserts a second user message.
		return nil
	case model.EventRunResumed:
		if row.Status != string(model.RunRecovering) {
			return run.ErrRunNotRunning
		}
		return w.tx.Table("threads").Where("id = ?", w.head.ID).Updates(map[string]any{"suggest_version": gorm.Expr("suggest_version + 1"), "next_input_suggestions_json": nil}).Error
	case model.EventRunProgress:
		if row.CancelRequestedAt != nil {
			return nil
		}
		return set("progress_json", raw)
	case model.EventContextWindowUpdated:
		var value model.ContextWindowUpdatedPayload
		if err := json.Unmarshal(update.Payload, &value); err != nil {
			return err
		}
		// Detailed token attribution belongs to its explicit inspector endpoint.
		value.Details = nil
		var thread struct{ PlanRevision int64 }
		if err := w.tx.Table("threads").Select("plan_revision").Where("id = ?", w.head.ID).Take(&thread).Error; err != nil {
			return err
		}
		encoded, err := json.Marshal(struct {
			model.ContextWindowUpdatedPayload
			ContextRevision int64 `json:"context_revision,string"`
			PlanRevision    int64 `json:"plan_revision,string"`
		}{value, w.head.ContextRevision, thread.PlanRevision})
		if err != nil {
			return err
		}
		return set("context_window_json", string(encoded))
	case model.EventRunUsageUpdated:
		return w.applyRunUsage(row, update.Payload)
	case model.EventPlanUpdated:
		var value model.PlanUpdatedPayload
		if err := json.Unmarshal(update.Payload, &value); err != nil {
			return err
		}
		var plan any
		if value.Plan != nil {
			encoded, err := json.Marshal(value.Plan)
			if err != nil {
				return err
			}
			plan = string(encoded)
		}
		return w.tx.Table("threads").Where("id = ?", w.head.ID).Updates(map[string]any{"plan_json": plan, "plan_run_id": nullableID(row.ID), "plan_revision": gorm.Expr("plan_revision + 1")}).Error
	case model.EventScopeUpdated:
		var value model.ScopeUpdatedPayload
		if err := json.Unmarshal(update.Payload, &value); err != nil {
			return err
		}
		// The checkpoint has already committed the effective scope. Public facts
		// cannot independently grant permissions or overwrite a newer revision.
		current, err := commandHash(row.Command.Scope)
		if err != nil {
			return err
		}
		requested, err := commandHash(value.Scope)
		if err != nil {
			return err
		}
		if current != requested {
			return sessionstate.ErrVersionConflict
		}
		return nil
	case model.EventRunModeChanged:
		var value model.RunModeChangedPayload
		if err := json.Unmarshal(update.Payload, &value); err != nil {
			return err
		}
		if row.Mode != string(value.Mode) {
			return sessionstate.ErrVersionConflict
		}
		return nil
	case model.EventMessageThinking, model.EventMessageMilestone:
		var value model.MessageThinkingPayload
		if err := json.Unmarshal(update.Payload, &value); err != nil {
			return err
		}
		kind := "thinking"
		if update.Type == model.EventMessageMilestone {
			kind = "message"
		}
		return w.putPublicItem(sessionstate.TimelineItem{ID: "message:" + row.ID + ":" + value.MessageID, ThreadID: w.head.ID, GroupID: "run:" + row.ID, Kind: kind}, update.Payload)
	case model.EventMessageFinal:
		return w.applyFinalMessage(row, update.Payload)
	case model.EventToolStarted, model.EventToolCompleted:
		var value struct {
			CallID string `json:"call_id"`
			Tool   string `json:"tool"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(update.Payload, &value); err != nil {
			return err
		}
		id := "tool:" + row.ID + ":" + value.CallID
		var old timelineItemPO
		err := w.tx.Where("id = ? AND thread_id = ?", id, w.head.ID).Take(&old).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			var saved struct {
				Tool   string `json:"tool"`
				Status string `json:"status"`
			}
			if json.Unmarshal([]byte(old.SummaryJSON), &saved) != nil || saved.Tool != value.Tool {
				return sessionstate.ErrInvalidState
			}
			if saved.Status != "running" && update.Type == model.EventToolStarted {
				return nil
			}
			if saved.Status != "running" && saved.Status != value.Status {
				return sessionstate.ErrVersionConflict
			}
		} else if update.Type == model.EventToolCompleted && value.Status != "blocked" {
			return sessionstate.ErrInvalidState
		}
		public := update.Payload
		if update.Type == model.EventToolStarted {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(public, &fields); err != nil {
				return err
			}
			fields["status"] = json.RawMessage(`"running"`)
			public, err = json.Marshal(fields)
			if err != nil {
				return err
			}
		}
		return w.putPublicItem(sessionstate.TimelineItem{ID: id, ThreadID: w.head.ID, GroupID: "run:" + row.ID, Kind: "tool", ToolCallID: value.CallID}, public)
	case model.EventContextCompacted:
		var value model.ContextCompactedPayload
		if err := json.Unmarshal(update.Payload, &value); err != nil {
			return err
		}
		return w.putPublicItem(sessionstate.TimelineItem{ID: "compaction:" + value.Compaction.ID, ThreadID: w.head.ID, GroupID: "run:" + row.ID, Kind: "compaction"}, update.Payload)
	case model.EventPreviewUpdated:
		// Preview data is part of an existing semantic tool item; it never creates
		// another completed tool or changes execution authority.
		var value model.PreviewUpdatedPayload
		if err := json.Unmarshal(update.Payload, &value); err != nil {
			return err
		}
		var item timelineItemPO
		if err := w.tx.Where("id = ? AND thread_id = ?", "tool:"+value.SourceRunID+":"+value.CallID, w.head.ID).Take(&item).Error; err != nil {
			return mapErr(err)
		}
		public, err := w.itemPublic(item)
		if err != nil {
			return err
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(public, &fields); err != nil {
			return err
		}
		fields["preview_assessment"], err = json.Marshal(value.Preview)
		if err != nil {
			return err
		}
		public, err = json.Marshal(fields)
		if err != nil {
			return err
		}
		return w.putPublicItem(item.view(), public)
	case model.EventRunCompleted, model.EventRunFailed, model.EventRunCanceled, model.EventRunError:
		return w.applyRunTerminal(row, update.Type, update.Payload)
	default:
		// Interaction requests/answers and direct input replies have their own typed
		// transactions. Routing them through generic display publication is invalid.
		return fmt.Errorf("%w: unsupported run update %s", sessionstate.ErrInvalidState, update.Type)
	}
}

func (w *sessionWrite) applyFinalMessage(row runPO, raw json.RawMessage) error {
	var value model.MessageFinalPayload
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if row.CancelRequestedAt != nil || row.FinalMessageID != nil {
		return sessionstate.ErrVersionConflict
	}
	id := value.MessageID
	var existing struct {
		ThreadID   string
		RunID      string
		PublicJSON *string
	}
	err := w.tx.Table("conversation_messages").Select("thread_id,run_id,public_json").Where("id = ?", id).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		roundID, err := w.openConversationRound(row.ID)
		if err != nil {
			return err
		}
		if err := w.insertMessages([]sessionstate.Message{{ID: id, RunID: row.ID, RoundID: roundID, Kind: "assistant", Native: llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent(value.Text)}, Public: raw}}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if existing.ThreadID != w.head.ID || existing.RunID != row.ID || existing.PublicJSON == nil || *existing.PublicJSON != string(raw) {
		return sessionstate.ErrInvalidState
	}
	if err := w.tx.Table("runs").Where("id = ?", row.ID).Update("final_message_id", id).Error; err != nil {
		return err
	}
	return w.putPublicItem(sessionstate.TimelineItem{ID: "message:" + row.ID + ":" + id, ThreadID: w.head.ID, GroupID: "run:" + row.ID, Kind: "final", MessageID: id}, raw)
}

func (w *sessionWrite) applyRunTerminal(row runPO, kind model.EventType, raw json.RawMessage) error {
	var value model.RunTerminalPayload
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	status := model.RunFailed
	switch kind {
	case model.EventRunCompleted:
		status = model.RunDone
	case model.EventRunCanceled:
		status = model.RunCanceled
		if row.CancelReason != nil {
			value.Reason = model.RunCancelReason(*row.CancelReason)
			var err error
			raw, err = json.Marshal(value)
			if err != nil {
				return err
			}
		}
	}
	if model.RunStatus(row.Status).Terminal() && row.Status != string(status) {
		return run.ErrRunNotRunning
	}
	if status == model.RunDone && (row.CancelRequestedAt != nil || row.FinalMessageID == nil) {
		return sessionstate.ErrVersionConflict
	}
	if row.TerminalJSON != nil {
		return nil
	}
	if value.SuppressSuggestions {
		if err := w.tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&idempotencyPO{Scope: "automatic_suggestions", OwnerID: row.ID, Key: "suppressed", RequestHash: "direct_reply", Status: "completed", ResultJSON: `{"suppressed":true}`}).Error; err != nil {
			return err
		}
	}
	updates := map[string]any{"terminal_json": string(raw), "active_duration_ms": value.DurationMS, "active_since": nil, "progress_json": nil}
	itemID := fmt.Sprintf("terminal:%s:%d", row.ID, row.ExecutionRevision)
	if err := w.putPublicItem(sessionstate.TimelineItem{ID: itemID, ThreadID: w.head.ID, GroupID: "run:" + row.ID, Kind: "terminal_notice"}, raw); err != nil {
		return err
	}
	var item timelineItemPO
	if err := w.tx.Select("summary_json,detail_ref").Where("id = ?", itemID).Take(&item).Error; err != nil {
		return err
	}
	updates["terminal_json"], updates["terminal_ref"] = item.SummaryJSON, item.DetailRef
	if kind == model.EventRunError || value.Reason == model.RunCancelSuperseded || value.Reason == model.RunCancelInputRejected || value.Reason == model.RunCancelDetectionFailed {
		updates["continuation_allowed"] = 0
	}
	if err := w.tx.Table("runs").Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return err
	}
	return w.setRunStatus(row, status)
}

// Usage totals have a bounded current representation. The commit receipt keyed
// by call_id prevents an at-least-once usage notification from double counting.
func (w *sessionWrite) applyRunUsage(row runPO, raw json.RawMessage) error {
	var value model.RunUsageUpdatedPayload
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	var total struct {
		model.TokenUsage
		Calls           int64 `json:"calls"`
		UnreportedCalls int64 `json:"unreported_calls"`
		DurationMS      int64 `json:"duration_ms"`
	}
	if row.UsageJSON != nil {
		if err := json.Unmarshal([]byte(*row.UsageJSON), &total); err != nil {
			return err
		}
	}
	total.Calls++
	total.DurationMS += value.Call.DurationMS
	if value.Call.Usage == nil {
		total.UnreportedCalls++
	} else {
		total.InputTokens += value.Call.Usage.InputTokens
		total.OutputTokens += value.Call.Usage.OutputTokens
		total.TotalTokens += value.Call.Usage.TotalTokens
	}
	encoded, err := json.Marshal(total)
	if err != nil {
		return err
	}
	return w.tx.Table("runs").Where("id = ?", row.ID).Update("usage_json", string(encoded)).Error
}

func (w *sessionWrite) itemPublic(item timelineItemPO) (json.RawMessage, error) {
	if item.DetailRef == nil {
		return json.RawMessage(item.SummaryJSON), nil
	}
	ref, err := sessionstate.DecodeContentRef(*item.DetailRef)
	if err != nil {
		return nil, err
	}
	var row struct{ PublicJSON string }
	if err := w.tx.Table("timeline_details").Where("id = ? AND thread_id = ?", ref.SourceID, w.head.ID).Take(&row).Error; err != nil {
		return nil, err
	}
	return json.RawMessage(row.PublicJSON), nil
}

func (w *sessionWrite) putPublicItem(item sessionstate.TimelineItem, public json.RawMessage) error {
	item.Summary, item.DetailRef, item.BodyRef = public, "", ""
	if len(public) > 32*1024 {
		detailID := model.MustShortID("detail")
		if err := w.tx.Table("timeline_details").Create(map[string]any{"id": detailID, "thread_id": w.head.ID, "public_json": string(public), "created_seq": w.seq}).Error; err != nil {
			return err
		}
		var revision int64
		if err := w.tx.Table("timeline_items").Select("COALESCE(MAX(entity_revision),0)").Where("id = ? AND thread_id = ?", item.ID, w.head.ID).Scan(&revision).Error; err != nil {
			return err
		}
		item.DetailRef = sessionstate.ContentRef{ThreadID: w.head.ID, ItemID: item.ID, SceneRevision: w.head.SceneRevision, ItemRevision: revision + 1, SourceType: "detail", SourceID: detailID}.Encode()
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(public, &fields); err != nil {
			return err
		}
		summary := map[string]json.RawMessage{}
		for _, key := range []string{"schema_version", "run_id", "occurred_at", "message_id", "call_id", "tool", "status", "duration_ms", "reason", "suppress_suggestions", "type", "runId", "deliveryStatus", "clientMessageId", "mode", "steering"} {
			if value, ok := fields[key]; ok {
				summary[key] = value
			}
		}
		if value, ok := fields["text"]; ok {
			var text string
			if json.Unmarshal(value, &text) != nil {
				return sessionstate.ErrInvalidState
			}
			chars := []rune(text)
			if len(chars) > 512 {
				chars = chars[:512]
			}
			summary["excerpt"], _ = json.Marshal(string(chars))
		}
		encoded, err := json.Marshal(summary)
		if err != nil {
			return err
		}
		item.Summary = encoded
	}
	return w.putItem(item)
}
