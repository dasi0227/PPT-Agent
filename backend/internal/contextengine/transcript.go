package contextengine

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

type TranscriptEntry struct {
	Role       llm.Role          `json:"role"`
	Content    []llm.ContentPart `json:"content"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []llm.ToolCall    `json:"tool_calls,omitempty"`
	Type       ContextBucket     `json:"type"`
	Layer      ContextLayer      `json:"layer"`
}

func (entry TranscriptEntry) Message() llm.Message {
	return llm.Message{
		Role: entry.Role, Content: append([]llm.ContentPart(nil), entry.Content...),
		ToolCallID: entry.ToolCallID, ToolCalls: append([]llm.ToolCall(nil), entry.ToolCalls...),
	}
}

type FSTranscriptStore struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewFSTranscriptStore() *FSTranscriptStore {
	return &FSTranscriptStore{locks: map[string]*sync.Mutex{}}
}

func TranscriptPath(threadID string) string {
	return filepath.ToSlash(filepath.Join("threads", threadID+".transcript.jsonl"))
}

func (s *FSTranscriptStore) Create(workDir, threadID string) error {
	return s.Replace(workDir, threadID, []llm.Message{})
}

func (s *FSTranscriptStore) Remove(workDir, threadID string) error {
	lock := s.lockFor(workDir, threadID)
	lock.Lock()
	defer lock.Unlock()
	err := os.Remove(filepath.Join(workDir, filepath.FromSlash(TranscriptPath(threadID))))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *FSTranscriptStore) Load(workDir, threadID string) ([]llm.Message, error) {
	entries, err := s.LoadEntries(workDir, threadID)
	if err != nil {
		return nil, err
	}
	messages := make([]llm.Message, 0, len(entries))
	for _, entry := range entries {
		messages = append(messages, entry.Message())
	}
	return messages, nil
}

func (s *FSTranscriptStore) LoadEntries(workDir, threadID string) ([]TranscriptEntry, error) {
	lock := s.lockFor(workDir, threadID)
	lock.Lock()
	defer lock.Unlock()
	path := filepath.Join(workDir, filepath.FromSlash(TranscriptPath(threadID)))
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []TranscriptEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries := []TranscriptEntry{}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry TranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, err
		}
		if entry.Layer != LayerTranscript {
			return nil, errors.New("transcript entry must use transcript layer")
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func (s *FSTranscriptStore) Replace(workDir, threadID string, messages []llm.Message) error {
	lock := s.lockFor(workDir, threadID)
	lock.Lock()
	defer lock.Unlock()
	path := filepath.Join(workDir, filepath.FromSlash(TranscriptPath(threadID)))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	entries := classifyTranscript(messages)
	var output strings.Builder
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".transcript-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.WriteString(output.String()); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func (s *FSTranscriptStore) lockFor(workDir, threadID string) *sync.Mutex {
	key := filepath.Join(workDir, threadID)
	s.mu.Lock()
	defer s.mu.Unlock()
	lock := s.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		s.locks[key] = lock
	}
	return lock
}

func classifyTranscript(messages []llm.Message) []TranscriptEntry {
	toolNames := map[string]string{}
	entries := make([]TranscriptEntry, 0, len(messages))
	for _, message := range messages {
		for _, call := range message.ToolCalls {
			toolNames[call.ID] = call.Name
		}
		bucket := BucketChatHistory
		if message.Role == llm.RoleUser {
			bucket = BucketUserPrompt
		}
		if message.Role == llm.RoleTool {
			switch toolNames[message.ToolCallID] {
			case "read_ppt":
				bucket = BucketReadPPT
			case "run_command":
				bucket = BucketRunCommand
			}
		}
		entries = append(entries, TranscriptEntry{
			Role: message.Role, Content: append([]llm.ContentPart(nil), message.Content...),
			ToolCallID: message.ToolCallID, ToolCalls: append([]llm.ToolCall(nil), message.ToolCalls...),
			Type: bucket, Layer: LayerTranscript,
		})
	}
	return entries
}

func loadTranscriptTurns(workDir, threadID string, limit int) []RecentTurn {
	entries, err := NewFSTranscriptStore().LoadEntries(workDir, threadID)
	if err != nil {
		return []RecentTurn{}
	}
	out := []RecentTurn{}
	for _, entry := range entries {
		if entry.Role != llm.RoleUser && entry.Role != llm.RoleAssistant {
			continue
		}
		text := compactText(entry.Message().Text(), 500)
		if text != "" {
			out = append(out, RecentTurn{Turn: string(entry.Role), Type: string(entry.Type), Text: text})
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}
