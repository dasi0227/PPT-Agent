package run

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// HistoryEntry 是 <workdir>/<history_path> 里 jsonl 每行的 schema（UX §2.3）。
type HistoryEntry struct {
	Seq   int64          `json:"seq"`
	TS    int64          `json:"ts"`
	RunID string         `json:"run_id"`
	Turn  string         `json:"turn"` // "user" | "agent"
	Type  string         `json:"type"` // user_turn | markdown | info | needs_input | final_result | error
	Data  map[string]any `json:"data"`
}

// HistoryWriter 抽象 history append 副作用；Bus 只依赖此接口，便于测试注入 nil / stub。
type HistoryWriter interface {
	Append(ctx context.Context, threadID string, entry HistoryEntry) error
}

// ThreadLocator 解耦 store：只暴露 append 需要的两个查询，避免反向依赖 store 全接口。
type ThreadLocator interface {
	GetThread(ctx context.Context, id string) (model.Thread, error)
	GetProject(ctx context.Context, id string) (model.Project, error)
}

// FSHistoryWriter 按 thread 独立 mutex 串行化 append 到 <workdir>/<history_path>。
// 目录不存在会自动创建；单 thread 内保证行的原子性与顺序，跨 thread 无相互阻塞。
type FSHistoryWriter struct {
	loc ThreadLocator

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewFSHistoryWriter(loc ThreadLocator) *FSHistoryWriter {
	return &FSHistoryWriter{loc: loc, locks: map[string]*sync.Mutex{}}
}

func (w *FSHistoryWriter) mutexFor(threadID string) *sync.Mutex {
	w.mu.Lock()
	defer w.mu.Unlock()
	m, ok := w.locks[threadID]
	if !ok {
		m = &sync.Mutex{}
		w.locks[threadID] = m
	}
	return m
}

// Append 追加一行 jsonl。失败即返回，由调用方决定是否吞掉（Bus 会吞掉以不阻塞 SSE）。
func (w *FSHistoryWriter) Append(ctx context.Context, threadID string, entry HistoryEntry) error {
	th, err := w.loc.GetThread(ctx, threadID)
	if err != nil {
		return err
	}
	proj, err := w.loc.GetProject(ctx, th.ProjectID)
	if err != nil {
		return err
	}
	full := filepath.Join(proj.WorkDir, filepath.FromSlash(th.HistoryPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	m := w.mutexFor(threadID)
	m.Lock()
	defer m.Unlock()

	f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(raw)
	return err
}
