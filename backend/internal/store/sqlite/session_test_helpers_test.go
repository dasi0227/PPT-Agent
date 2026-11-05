package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

func runContextForTest(t *testing.T, s *Store, id string) context.Context {
	t.Helper()
	r, err := s.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return workflow.WithCheckpointLease(sessionstate.WithRequestScene(context.Background(), r.SceneRevision), r.OwnerInstanceID, r.ExecutionRevision, r.CheckpointRevision)
}

func runUpdateForTest(t *testing.T, s *Store, id string, kind model.EventType, payload any) sessionstate.Receipt {
	t.Helper()
	update, err := sessionstate.NewRunUpdate(kind, payload)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.ApplyRunUpdate(runContextForTest(t, s, id), id, update)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

// Reader fixtures use the same current message/round representation as runtime
// commits. They deliberately do not create or replay a parallel event journal.
func conversationMessageForTest(t *testing.T, s *Store, runID, text string, public bool) {
	t.Helper()
	current, err := s.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	id := model.MustShortID("msg")
	_, err = s.commitSession(context.Background(), sessionstate.Fence{ProjectID: current.ProjectID, ThreadID: current.ThreadID, SceneRevision: current.SceneRevision}, sessionstate.CommitIdentity{Key: id, RequestHash: text}, func(w *sessionWrite) error {
		round, err := w.openConversationRound(runID)
		if err != nil {
			return err
		}
		message := sessionstate.Message{ID: id, RunID: runID, RoundID: round, Kind: "assistant", Native: llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent(text)}}
		if public {
			message.Public, err = json.Marshal(map[string]string{"text": text})
		}
		if err != nil {
			return err
		}
		return w.insertMessages([]sessionstate.Message{message})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	cfg := &config.RuntimeConfig{WorkRoot: root, DBPath: filepath.Join(root, "test.db")}
	db, cleanup, err := Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	s, err := NewStore(db)
	if err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	return s
}

func activeRunSpec() model.RunCommand {
	return model.RunCommand{
		Scope:       model.NewRunScope(model.ScopeAllPages),
		Mode:        model.ModeCreate,
		Instruction: "test",
	}
}

func currentRunAction(t *testing.T, s *Store, id, key string) sessionstate.RunAction {
	t.Helper()
	r, err := s.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return sessionstate.RunAction{SceneRevision: r.SceneRevision, EntityRevision: r.EntityRevision, RequestKey: key}
}

func resumeRunForTest(t *testing.T, s *Store, id, owner, key string) (sessionstate.RunMutation, error) {
	t.Helper()
	current, err := s.GetRun(context.Background(), id)
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	result, _, err := s.ClaimRunForResume(context.Background(), current, owner, sessionstate.RunAction{SceneRevision: current.SceneRevision, EntityRevision: current.EntityRevision, RequestKey: key})
	return result, err
}

func steeringForTest(t *testing.T, s *Store, runID, id, content string) model.SteeringMessage {
	t.Helper()
	r, err := s.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	message, _, err := s.CreateSteering(sessionstate.WithRequestScene(context.Background(), r.SceneRevision), model.SteeringMessage{RunID: runID, ThreadID: r.ThreadID, ClientMessageID: id, RequestHash: id, Content: content, Scope: r.Command.Scope})
	if err != nil {
		t.Fatal(err)
	}
	return message
}
