package contextengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

type TranscriptEntry struct {
	Role       llm.Role             `json:"role"`
	Content    []llm.ContentPart    `json:"content"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	ToolCalls  []llm.ToolCall       `json:"tool_calls,omitempty"`
	Metadata   *llm.MessageMetadata `json:"metadata,omitempty"`
}

func (entry TranscriptEntry) Message() llm.Message {
	return llm.Message{
		Role: entry.Role, Content: append([]llm.ContentPart(nil), entry.Content...),
		ToolCallID: entry.ToolCallID, ToolCalls: append([]llm.ToolCall(nil), entry.ToolCalls...),
		Metadata: entry.Metadata,
	}
}

// JournalTranscriptStore projects model messages from the authoritative log.
// Reading can use files directly; mutations always go through the durable outbox.
type JournalTranscriptStore struct {
	mu      sync.Mutex
	backend threadjournal.Backend
}

func NewJournalTranscriptStore(backend threadjournal.Backend) *JournalTranscriptStore {
	return &JournalTranscriptStore{backend: backend}
}

type projectedEntry struct {
	ID    string          `json:"id"`
	Entry TranscriptEntry `json:"entry"`
}
type modelEdit struct {
	Watermark int64            `json:"watermark"`
	Remove    []string         `json:"remove"`
	Before    string           `json:"before,omitempty"`
	Entries   []projectedEntry `json:"entries"`
}

func (s *JournalTranscriptStore) events(workDir, threadID string) ([]threadjournal.Event, error) {
	if s.backend != nil {
		return s.backend.ThreadEvents(context.Background(), threadID, 0)
	}
	return threadjournal.Read(model.ProjectRoot(workDir), filepath.Base(model.ProjectRoot(workDir)), threadID)
}
func projectMessages(events []threadjournal.Event) ([]projectedEntry, error) {
	out := []projectedEntry{}
	known := map[string]bool{}
	for _, event := range events {
		if event.Type != "model.edit" {
			continue
		}
		var edit modelEdit
		if err := json.Unmarshal(event.Payload, &edit); err != nil {
			return nil, err
		}
		if edit.Watermark >= event.Seq {
			return nil, errors.New("model edit has an invalid watermark")
		}
		active := map[string]bool{}
		for _, entry := range out {
			active[entry.ID] = true
		}
		remove := map[string]bool{}
		for _, id := range edit.Remove {
			if !active[id] || remove[id] {
				return nil, errors.New("model edit refers to unknown message")
			}
			remove[id] = true
		}
		next := make([]projectedEntry, 0, len(out)+len(edit.Entries))
		inserted := false
		for _, entry := range edit.Entries {
			if entry.ID == "" || known[entry.ID] {
				return nil, errors.New("invalid model message identity")
			}
			known[entry.ID] = true
		}
		for _, entry := range out {
			if entry.ID == edit.Before {
				next = append(next, edit.Entries...)
				inserted = true
			}
			if !remove[entry.ID] {
				next = append(next, entry)
			}
		}
		if !inserted {
			if edit.Before != "" {
				return nil, errors.New("model edit insertion anchor is missing")
			}
			next = append(next, edit.Entries...)
		}
		out = next
	}
	return out, nil
}
func (s *JournalTranscriptStore) Load(workDir, threadID string) ([]llm.Message, error) {
	entries, err := s.LoadEntries(workDir, threadID)
	if err != nil {
		return nil, err
	}
	out := make([]llm.Message, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Message())
	}
	return llm.NormalizeHistory(out), nil
}
func (s *JournalTranscriptStore) LoadEntries(workDir, threadID string) ([]TranscriptEntry, error) {
	events, err := s.events(workDir, threadID)
	if err != nil {
		return nil, err
	}
	entries, err := projectMessages(events)
	if err != nil {
		return nil, err
	}
	out := make([]TranscriptEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Entry)
	}
	return out, nil
}
func sameEntry(a, b TranscriptEntry) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func (s *JournalTranscriptStore) Replace(workDir, threadID string, messages []llm.Message) error {
	return s.replace(context.Background(), "", workDir, threadID, nil, messages)
}

func (s *JournalTranscriptStore) ReplaceFromContext(ctx context.Context, workDir, threadID string, original, messages []llm.Message) error {
	return s.replace(ctx, "", workDir, threadID, transcriptEntries(llm.NormalizeHistory(original)), messages)
}
func (s *JournalTranscriptStore) replace(ctx context.Context, runID, workDir, threadID string, original []TranscriptEntry, messages []llm.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend == nil {
		return errors.New("model history writes require the journal backend")
	}
	events, err := s.events(workDir, threadID)
	if err != nil {
		return err
	}
	current, err := projectMessages(events)
	if err != nil {
		return err
	}
	old := current
	if original != nil {
		if len(original) > len(current) {
			return errors.New("compression input is no longer current")
		}
		for i, e := range original {
			if !sameEntry(e, current[i].Entry) {
				return errors.New("compression input changed")
			}
		}
		old = current[:len(original)]
	}
	next := transcriptEntries(llm.NormalizeHistory(messages))
	prefix := 0
	for prefix < len(old) && prefix < len(next) && sameEntry(old[prefix].Entry, next[prefix]) {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(next)-prefix && sameEntry(old[len(old)-1-suffix].Entry, next[len(next)-1-suffix]) {
		suffix++
	}
	if prefix == len(old) && prefix == len(next) {
		return nil
	}
	edit := modelEdit{Remove: []string{}, Entries: []projectedEntry{}}
	if len(events) > 0 {
		edit.Watermark = events[len(events)-1].Seq
	}
	for _, e := range old[prefix : len(old)-suffix] {
		edit.Remove = append(edit.Remove, e.ID)
	}
	if suffix > 0 {
		edit.Before = old[len(old)-suffix].ID
	} else if len(old) < len(current) {
		edit.Before = current[len(old)].ID
	}
	for _, e := range next[prefix : len(next)-suffix] {
		edit.Entries = append(edit.Entries, projectedEntry{ID: model.MustShortID("msg"), Entry: e})
	}
	raw, err := json.Marshal(edit)
	if err != nil {
		return err
	}
	_, err = s.backend.AppendThreadEvent(ctx, threadID, threadjournal.Event{Type: "model.edit", RunID: runID, Payload: raw})
	return err
}

func (s *JournalTranscriptStore) ReplaceForRun(ctx context.Context, runID, workDir, threadID string, messages []llm.Message) error {
	return s.replace(ctx, runID, workDir, threadID, nil, messages)
}

// Conversation storage keeps provenance, not a presentation bucket. Accounting
// classifies the current request per content block, including mixed tool results.
func transcriptEntries(messages []llm.Message) []TranscriptEntry {
	entries := make([]TranscriptEntry, 0, len(messages))
	for _, message := range messages {
		entries = append(entries, TranscriptEntry{
			Role: message.Role, Content: append([]llm.ContentPart(nil), message.Content...),
			ToolCallID: message.ToolCallID, ToolCalls: append([]llm.ToolCall(nil), message.ToolCalls...),
			Metadata: message.Metadata,
		})
	}
	return entries
}
