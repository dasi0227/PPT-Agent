package sqlite

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

// Explicit document commands can read the full public conversation. Normal
// classifiers and UI paths use the bounded recent reader below.
func (s *Store) LoadThreadConversationContext(ctx context.Context, threadID string) (model.ThreadConversationContext, error) {
	return s.readConversation(ctx, threadID, "", "", 0, false)
}
func (s *Store) LoadRecentThreadConversationContext(ctx context.Context, threadID string) (model.ThreadConversationContext, error) {
	return s.readConversation(ctx, threadID, "", "", contextengine.DecisionHistoryWindow().MaxRounds, false)
}
func (s *Store) LoadThreadConversationContextBeforeInput(ctx context.Context, threadID, runID, inputID string) (model.ThreadConversationContext, error) {
	return s.readConversation(ctx, threadID, runID, inputID, contextengine.DecisionHistoryWindow().MaxRounds, false)
}
func (s *Store) LoadTaskConversationBeforeInput(ctx context.Context, threadID, runID, inputID string) (model.ThreadConversationContext, error) {
	return s.readConversation(ctx, threadID, runID, inputID, contextengine.DecisionHistoryWindow().MaxRounds, true)
}

func (s *Store) readConversation(ctx context.Context, threadID, runID, inputID string, roundLimit int, taskOnly bool) (model.ThreadConversationContext, error) {
	out := model.ThreadConversationContext{Rounds: []model.ConversationRound{}}
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		before := head.LastSeq + 1
		if inputID != "" {
			input, err := findRunInput(tx, threadID, runID, inputID)
			if err != nil {
				return err
			}
			if taskOnly && input.Disposition != model.InputTask {
				return sessionstate.ErrInvalidState
			}
			before = input.HistoryBeforeSeq
			var command model.RunCommand
			if err := json.Unmarshal([]byte(input.ReferenceJSON), &command); err != nil {
				return err
			}
			out.Input = &model.ConversationInput{RunID: runID, InputID: inputID, AcceptedSeq: input.AcceptedSeq, Text: command.Instruction,
				HasExplicitReferences: len(command.Attachments)+len(command.DOMSelections)+len(command.ReferenceOrder)+len(command.Skills)+len(command.Components)+len(command.MentionedPages)+len(command.DroppedMentionedSlideIDs) > 0}
		}
		selected := tx.Session(&gorm.Session{NewDB: true}).Table("conversation_rounds").Select("id,opened_seq,has_task_input").
			Where("thread_id = ? AND opened_seq < ? AND public_message_count > 0", threadID, before).Order("opened_seq DESC, id DESC")
		if taskOnly {
			selected = selected.Where("has_task_input = 1")
		}
		if roundLimit > 0 {
			selected = selected.Limit(roundLimit)
		}
		var rows []struct {
			RoundID string
			Role    string
			Text    string
		}
		query := tx.Table("conversation_messages AS m").Select("m.round_id, m.role, COALESCE(json_extract(m.public_json, '$.text'), '') AS text").
			Joins("JOIN (?) AS selected ON selected.id = m.round_id", selected).
			Where("m.thread_id = ? AND m.created_seq < ? AND m.public_json IS NOT NULL AND m.role IN ('user','assistant')", threadID, before)
		if taskOnly {
			query = query.Joins("LEFT JOIN thread_inputs AS input ON input.id = m.input_id AND input.thread_id = m.thread_id").
				Where("input.disposition = ? OR (m.input_id IS NULL AND selected.has_task_input = 1 AND m.kind <> 'direct_reply')", model.InputTask)
		}
		if err := query.Order("selected.opened_seq ASC, selected.id ASC, m.created_seq ASC, m.id ASC").Scan(&rows).Error; err != nil {
			return err
		}
		previous := ""
		for _, row := range rows {
			if row.Text == "" {
				continue
			}
			if previous != row.RoundID {
				out.Rounds = append(out.Rounds, model.ConversationRound{})
				previous = row.RoundID
			}
			round := &out.Rounds[len(out.Rounds)-1]
			round.Messages = append(round.Messages, model.ConversationMessage{Role: row.Role, Text: row.Text})
		}
		plan, err := readThreadPlan(tx, threadID)
		out.Plan = plan.Plan
		return err
	})
	return out, err
}

func readThreadPlan(tx *gorm.DB, threadID string) (model.ThreadPlanSnapshot, error) {
	var row struct {
		PlanJSON     *string
		PlanRunID    *string
		PlanRevision int64
	}
	var out model.ThreadPlanSnapshot
	if err := tx.Table("threads").Select("plan_json,plan_run_id,plan_revision").Where("id = ?", threadID).Take(&row).Error; err != nil {
		return out, mapErr(err)
	}
	out.RunID, out.Seq = textOrEmpty(row.PlanRunID), row.PlanRevision
	if row.PlanJSON != nil {
		if err := json.Unmarshal([]byte(*row.PlanJSON), &out.Plan); err != nil {
			return out, err
		}
	}
	return out, nil
}
func (s *Store) LatestThreadPlan(ctx context.Context, threadID string) (model.ThreadPlanSnapshot, error) {
	var out model.ThreadPlanSnapshot
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := readSessionHead(tx, threadID); err != nil {
			return err
		}
		var err error
		out, err = readThreadPlan(tx, threadID)
		return err
	})
	return out, err
}

func (s *Store) ClearThreadPlan(ctx context.Context, threadID string, expectedRevision int64, requestKey string) (sessionstate.Receipt, error) {
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return sessionstate.Receipt{}, sessionstate.ErrSceneChanged
	}
	if expectedRevision < 0 || requestKey == "" {
		return sessionstate.Receipt{}, sessionstate.ErrInvalidState
	}
	thread, err := s.GetThread(ctx, threadID)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	hash, err := commandHash(struct{ PlanRevision int64 }{expectedRevision})
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	return s.commitSession(ctx, sessionstate.Fence{ProjectID: thread.ProjectID, ThreadID: threadID, SceneRevision: scene},
		sessionstate.CommitIdentity{Key: "clear-plan:" + requestKey, RequestHash: hash}, func(w *sessionWrite) error {
			var busy int64
			if err := w.tx.Table("runs").Where("thread_id = ? AND status IN ?", threadID, []string{"pending", "running", "waiting", "recovering"}).Count(&busy).Error; err != nil {
				return err
			}
			if busy > 0 {
				return model.ErrThreadPlanBusy
			}
			current, err := readThreadPlan(w.tx, threadID)
			if err != nil {
				return err
			}
			if current.Seq != expectedRevision {
				return sessionstate.ErrVersionConflict
			}
			if current.Plan != nil {
				current.Seq++
				if err := w.tx.Table("threads").Where("id = ?", threadID).Updates(map[string]any{"plan_json": nil, "plan_run_id": nil, "plan_revision": current.Seq}).Error; err != nil {
					return err
				}
				if err := w.publishThread(); err != nil {
					return err
				}
				value, err := json.Marshal(struct {
					ContextRevision int64 `json:"context_revision,string"`
				}{w.head.ContextRevision})
				if err != nil {
					return err
				}
				w.change("context_window", sessionstate.Change{Type: "context_window.invalidate", Entity: value})
			}
			w.result, err = json.Marshal(struct {
				PlanRevision int64 `json:"plan_revision,string"`
			}{current.Seq})
			return err
		})
}
