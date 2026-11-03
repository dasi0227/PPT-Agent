package contextengine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

// ContextSession owns one worker's last committed context. Preparing a commit
// never reads storage or historical versions. Publish is called only after the
// database has committed both context and the corresponding checkpoint.
type ContextSession struct {
	Snapshot sessionstate.ContextSnapshot
	bodies   map[string][32]byte
}

type PreparedContext struct {
	Update   sessionstate.ContextUpdate
	Messages []llm.Message
	bodies   map[string][32]byte
}

func NewContextSession(snapshot sessionstate.ContextSnapshot) (*ContextSession, error) {
	session := &ContextSession{Snapshot: snapshot, bodies: make(map[string][32]byte, len(snapshot.Messages))}
	for i := range session.Snapshot.Messages {
		message := &session.Snapshot.Messages[i]
		if message.ID == "" {
			return nil, sessionstate.ErrInvalidState
		}
		if _, exists := session.bodies[message.ID]; exists {
			return nil, sessionstate.ErrInvalidState
		}
		raw, err := sessionstate.EncodeMessage(message.Native)
		if err != nil {
			return nil, err
		}
		message.Native.PersistentID = message.ID
		session.bodies[message.ID] = sha256.Sum256(raw)
	}
	return session, nil
}

func (s *ContextSession) Messages() []llm.Message {
	messages := make([]llm.Message, len(s.Snapshot.Messages))
	for i, entry := range s.Snapshot.Messages {
		messages[i] = entry.Native
	}
	return messages
}

func (s *ContextSession) Prepare(messages []llm.Message, runID, reason string) (PreparedContext, error) {
	prepared := PreparedContext{Update: sessionstate.ContextUpdate{ExpectedRevision: s.Snapshot.Revision, MessageIDs: []string{}, NewMessages: []sessionstate.Message{}, Reason: reason}, Messages: llm.NormalizeHistory(messages), bodies: make(map[string][32]byte, len(messages))}
	for i := range prepared.Messages {
		message := &prepared.Messages[i]
		raw, err := sessionstate.EncodeMessage(*message)
		if err != nil {
			return prepared, err
		}
		digest := sha256.Sum256(raw)
		stored, exists := s.bodies[message.PersistentID]
		_, duplicate := prepared.bodies[message.PersistentID]
		if message.PersistentID == "" || !exists || stored != digest || duplicate {
			// Native transformations (for example retiring a render image) create
			// a new immutable body; they must not mutate an earlier checkpoint.
			// The same candidate at the same base version has the same IDs even
			// when the caller retries after losing a commit acknowledgement.
			identity, err := json.Marshal(struct {
				ThreadID, RunID string
				Scene, Revision int64
				Position        int
				Body            [32]byte
			}{s.Snapshot.ThreadID, runID, s.Snapshot.SceneRevision, s.Snapshot.Revision, i, digest})
			if err != nil {
				return prepared, err
			}
			message.PersistentID = fmt.Sprintf("msg_%x", sha256.Sum256(identity))
			kind, owner := "assistant", runID
			if message.Metadata != nil && message.Metadata.RunID != "" {
				owner = message.Metadata.RunID
			}
			switch message.Role {
			case llm.RoleUser:
				kind = "instruction"
			case llm.RoleTool:
				kind = "tool_result"
			case llm.RoleAssistant:
			default:
				return prepared, sessionstate.ErrInvalidState
			}
			if message.Metadata != nil && message.Metadata.Kind == "summary" {
				kind = "summary"
			}
			roundID := owner
			if roundID == "" {
				roundID = "context:" + s.Snapshot.ThreadID
			}
			prepared.Update.NewMessages = append(prepared.Update.NewMessages, sessionstate.Message{ID: message.PersistentID, RunID: owner, RoundID: roundID, Kind: kind, Native: *message})
		}
		prepared.Update.MessageIDs = append(prepared.Update.MessageIDs, message.PersistentID)
		prepared.bodies[message.PersistentID] = digest
	}
	if prepared.Update.Reason == "" {
		prepared.Update.Reason = "append"
		if len(prepared.Update.MessageIDs) < len(s.Snapshot.Messages) {
			prepared.Update.Reason = "replace"
		} else {
			for i, before := range s.Snapshot.Messages {
				if before.ID != prepared.Update.MessageIDs[i] {
					prepared.Update.Reason = "replace"
					break
				}
			}
		}
	}
	return prepared, nil
}

func (s *ContextSession) Publish(prepared PreparedContext, receipt sessionstate.Receipt) error {
	if prepared.Update.ExpectedRevision != s.Snapshot.Revision || receipt.SceneRevision != s.Snapshot.SceneRevision || receipt.ContextRevision < s.Snapshot.Revision || receipt.ContextRevision > s.Snapshot.Revision+1 {
		return sessionstate.ErrInvalidState
	}
	before := make([]string, len(s.Snapshot.Messages))
	for i, message := range s.Snapshot.Messages {
		before[i] = message.ID
	}
	changed := !slices.Equal(before, prepared.Update.MessageIDs)
	if changed != (receipt.ContextRevision == s.Snapshot.Revision+1) {
		return sessionstate.ErrInvalidState
	}
	byID := make(map[string]sessionstate.Message, len(s.Snapshot.Messages)+len(prepared.Update.NewMessages))
	for _, message := range s.Snapshot.Messages {
		byID[message.ID] = message
	}
	for _, message := range prepared.Update.NewMessages {
		byID[message.ID] = message
	}
	next := make([]sessionstate.Message, len(prepared.Update.MessageIDs))
	for i, id := range prepared.Update.MessageIDs {
		message, ok := byID[id]
		if !ok {
			return sessionstate.ErrInvalidState
		}
		next[i] = message
	}
	s.Snapshot.Revision, s.Snapshot.Messages, s.bodies = receipt.ContextRevision, next, prepared.bodies
	return nil
}
