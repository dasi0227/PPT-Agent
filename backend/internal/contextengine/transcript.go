package contextengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

type TranscriptEntry struct {
	Replay     *llm.AssistantReplay `json:"assistant_replay,omitempty"`
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
		Metadata: entry.Metadata, Replay: entry.Replay,
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
	// Non-nil when the journal entry must be omitted or expanded in task context.
	taskSource []TranscriptEntry
}
type modelEdit struct {
	TaskOnly  bool             `json:"task_only,omitempty"` // The source snapshot excluded direct exchanges.
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

// A direct reply owns its immediately following model edit through the event
// watermark. Identify the exchange by its application-issued message identity.
func directReplyEdit(previous threadjournal.Event, event threadjournal.Event, edit modelEdit) (bool, error) {
	if previous.RunID != event.RunID || previous.Seq != edit.Watermark || len(edit.Remove) != 0 || len(edit.Entries) != 2 ||
		(previous.Type != string(model.EventMessageFinal) && previous.Type != string(model.EventMessageReply)) {
		return false, nil
	}
	var reply model.MessageReplyPayload
	if err := json.Unmarshal(previous.Payload, &reply); err != nil {
		return false, err
	}
	first := edit.Entries[0].Entry
	return strings.HasPrefix(reply.MessageID, "input_") && first.Role == llm.RoleUser && first.Metadata != nil &&
		first.Metadata.Origin == "user" && first.Metadata.Kind == "instruction" && first.Metadata.RunID == event.RunID &&
		edit.Entries[1].Entry.Role == llm.RoleAssistant, nil
}

func (entry projectedEntry) appendTaskSource(out []TranscriptEntry) []TranscriptEntry {
	if entry.taskSource != nil {
		return append(out, entry.taskSource...)
	}
	return append(out, entry.Entry)
}

func taskEntries(entries []projectedEntry) []TranscriptEntry {
	out := make([]TranscriptEntry, 0, len(entries))
	for _, entry := range entries {
		out = entry.appendTaskSource(out)
	}
	return out
}

func projectMessages(events []threadjournal.Event) ([]projectedEntry, error) {
	out := []projectedEntry{}
	known := map[string]bool{}
	for index, event := range events {
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
		mixed := false
		active := map[string]bool{}
		for _, entry := range out {
			active[entry.ID] = true
			mixed = mixed || entry.taskSource != nil
		}
		remove := map[string]bool{}
		for _, id := range edit.Remove {
			if !active[id] || remove[id] {
				return nil, errors.New("model edit refers to unknown message")
			}
			remove[id] = true
		}
		if !edit.TaskOnly && index > 0 {
			direct, err := directReplyEdit(events[index-1], event, edit)
			if err != nil {
				return nil, err
			}
			if direct {
				for i := range edit.Entries {
					edit.Entries[i].taskSource = []TranscriptEntry{}
				}
			}
		}
		// A summary that saw a direct reply cannot be separated by trimming
		// its prose. Replay the task records it replaced, in journal order.
		if !edit.TaskOnly && mixed && len(edit.Entries) != 0 {
			m := edit.Entries[0].Entry.Metadata
			if m != nil && m.Origin == "runtime" && m.Kind == "summary" {
				source := []TranscriptEntry{}
				for _, entry := range out {
					if remove[entry.ID] {
						source = entry.appendTaskSource(source)
					}
				}
				for i := range edit.Entries {
					edit.Entries[i].taskSource = []TranscriptEntry{}
				}
				edit.Entries[0].taskSource = source
			}
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
	return taskEntries(entries), nil
}
func (entry projectedEntry) matches(other TranscriptEntry) bool {
	return entry.taskSource == nil && sameEntry(entry.Entry, other)
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
	old, err := projectMessages(events)
	if err != nil {
		return err
	}
	next := transcriptEntries(llm.NormalizeHistory(messages))
	prefixLimit := min(len(old), len(next))
	if original != nil {
		current := taskEntries(old)
		if len(original) > len(current) {
			return errors.New("compression input is no longer current")
		}
		for i, e := range original {
			if !sameEntry(e, current[i]) {
				return errors.New("compression input changed")
			}
		}
		// Keep concurrently appended entries anchored, even if their content
		// equals the replacement. Only the original snapshot can share a prefix.
		prefixLimit = min(prefixLimit, len(original))
		next = append(next, current[len(original):]...)
	}
	prefix := 0
	for prefix < prefixLimit && old[prefix].matches(next[prefix]) {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(next)-prefix && old[len(old)-1-suffix].matches(next[len(next)-1-suffix]) {
		suffix++
	}
	if prefix == len(old) && prefix == len(next) {
		return nil
	}
	edit := modelEdit{TaskOnly: true, Remove: []string{}, Entries: []projectedEntry{}}
	if len(events) > 0 {
		edit.Watermark = events[len(events)-1].Seq
	}
	for _, e := range old[prefix : len(old)-suffix] {
		edit.Remove = append(edit.Remove, e.ID)
	}
	if suffix > 0 {
		edit.Before = old[len(old)-suffix].ID
	}
	for _, entry := range next[prefix : len(next)-suffix] {
		edit.Entries = append(edit.Entries, projectedEntry{ID: model.MustShortID("msg"), Entry: entry})
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
			Metadata: message.Metadata, Replay: message.Replay,
		})
	}
	return entries
}
