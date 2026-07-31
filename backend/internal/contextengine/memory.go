package contextengine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type MemoryItem struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	SourceRun  string `json:"source_run_id"`
	RecordedAt int64  `json:"recorded_at"`
	Supersedes string `json:"supersedes,omitempty"`
}

type ThreadMemory struct {
	SchemaVersion      string       `json:"schema_version"`
	Revision           int          `json:"revision"`
	UserPreferences    []MemoryItem `json:"user_preferences"`
	ConfirmedDecisions []MemoryItem `json:"confirmed_decisions"`
	BrandConstraints   []MemoryItem `json:"brand_constraints"`
	ContentFacts       []MemoryItem `json:"content_facts"`
	OpenQuestions      []MemoryItem `json:"open_questions"`
	RecentChanges      []MemoryItem `json:"recent_changes"`
}

func EmptyMemory() ThreadMemory {
	return ThreadMemory{SchemaVersion: SchemaVersion, UserPreferences: []MemoryItem{}, ConfirmedDecisions: []MemoryItem{},
		BrandConstraints: []MemoryItem{}, ContentFacts: []MemoryItem{}, OpenQuestions: []MemoryItem{}, RecentChanges: []MemoryItem{}}
}

type ThreadMemoryStore struct{}

func memoryPath(workDir, threadID string) string {
	return filepath.Join(workDir, "threads", threadID+".memory.json")
}

func (ThreadMemoryStore) Load(workDir, threadID string) (ThreadMemory, []string, error) {
	raw, err := os.ReadFile(memoryPath(workDir, threadID))
	if errors.Is(err, os.ErrNotExist) {
		return EmptyMemory(), nil, nil
	}
	if err != nil {
		return ThreadMemory{}, nil, err
	}
	var m ThreadMemory
	if json.Unmarshal(raw, &m) != nil || m.SchemaVersion != SchemaVersion || m.Revision < 0 {
		return EmptyMemory(), []string{"thread memory was corrupt and safely rebuilt"}, nil
	}
	normalizeMemory(&m)
	return m, nil, nil
}

func (ThreadMemoryStore) Save(workDir, threadID string, m ThreadMemory) error {
	normalizeMemory(&m)
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := memoryPath(workDir, threadID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".memory-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

type ThreadMemoryUpdater struct {
	Clock func() int64
}

func (u ThreadMemoryUpdater) UpdateSuccessful(old ThreadMemory, runID, instruction string) ThreadMemory {
	now := time.Now().Unix()
	if u.Clock != nil {
		now = u.Clock()
	}
	next := old
	next.SchemaVersion = SchemaVersion
	next.Revision++
	value := strings.TrimSpace(instruction)
	if len(value) > 500 {
		value = value[:500]
	}
	if value != "" {
		next.RecentChanges = mergeMemory(next.RecentChanges, MemoryItem{
			Key: "run:" + runID, Value: value, SourceRun: runID, RecordedAt: now,
		}, 20)
	}
	normalizeMemory(&next)
	return next
}

func mergeMemory(items []MemoryItem, add MemoryItem, limit int) []MemoryItem {
	out := make([]MemoryItem, 0, len(items)+1)
	for _, item := range items {
		if item.Key == add.Key {
			add.Supersedes = item.SourceRun
			continue
		}
		out = append(out, item)
	}
	out = append(out, add)
	sort.SliceStable(out, func(i, j int) bool { return out[i].RecordedAt < out[j].RecordedAt })
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func normalizeMemory(m *ThreadMemory) {
	if m.UserPreferences == nil {
		m.UserPreferences = []MemoryItem{}
	}
	if m.ConfirmedDecisions == nil {
		m.ConfirmedDecisions = []MemoryItem{}
	}
	if m.BrandConstraints == nil {
		m.BrandConstraints = []MemoryItem{}
	}
	if m.ContentFacts == nil {
		m.ContentFacts = []MemoryItem{}
	}
	if m.OpenQuestions == nil {
		m.OpenQuestions = []MemoryItem{}
	}
	if m.RecentChanges == nil {
		m.RecentChanges = []MemoryItem{}
	}
	m.UserPreferences = capItems(m.UserPreferences, 30)
	m.ConfirmedDecisions = capItems(m.ConfirmedDecisions, 40)
	m.BrandConstraints = capItems(m.BrandConstraints, 30)
	m.ContentFacts = capItems(m.ContentFacts, 60)
	m.OpenQuestions = capItems(m.OpenQuestions, 30)
	m.RecentChanges = capItems(m.RecentChanges, 20)
}

func capItems(items []MemoryItem, n int) []MemoryItem {
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}
