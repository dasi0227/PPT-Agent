package service

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// ThreadService 管理一个 project 下的对话线程与 history jsonl。
type ThreadService struct {
	store store.Store
	clock func() int64
	newID func() string
}

type CreateThreadParams struct {
	Title string
}

func NewThreadService(s store.Store) *ThreadService {
	return &ThreadService{store: s, clock: func() int64 { return time.Now().Unix() }, newID: uuid.NewString}
}

func (svc *ThreadService) CreateThread(ctx context.Context, projectID string, p CreateThreadParams) (model.Thread, error) {
	proj, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return model.Thread{}, err
	}
	id := svc.newID()
	now := svc.clock()
	th := model.Thread{
		ID:          id,
		ProjectID:   projectID,
		Title:       strings.TrimSpace(p.Title),
		HistoryPath: filepath.ToSlash(filepath.Join("threads", id+".jsonl")),
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := writeEmptyHistory(proj.WorkDir, th.HistoryPath); err != nil {
		return model.Thread{}, err
	}
	if err := svc.store.CreateThread(ctx, th); err != nil {
		_ = removeHistory(proj.WorkDir, th.HistoryPath)
		return model.Thread{}, err
	}
	return th, nil
}

func (svc *ThreadService) RenameThread(ctx context.Context, id, title string) (model.Thread, error) {
	t, err := svc.store.GetThread(ctx, id)
	if err != nil {
		return model.Thread{}, err
	}
	t.Title = title
	t.UpdatedAt = svc.clock()
	if err := svc.store.UpdateThreadTitle(ctx, t.ID, t.Title, t.UpdatedAt); err != nil {
		return model.Thread{}, err
	}
	return t, nil
}

func (svc *ThreadService) ListThreads(ctx context.Context, projectID string) ([]model.Thread, error) {
	if _, err := svc.store.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	return svc.store.ListThreads(ctx, projectID)
}

func (svc *ThreadService) GetThread(ctx context.Context, id string) (model.Thread, error) {
	return svc.store.GetThread(ctx, id)
}

func (svc *ThreadService) DeleteThread(ctx context.Context, id string) error {
	th, err := svc.store.GetThread(ctx, id)
	if err != nil {
		return err
	}
	proj, err := svc.store.GetProject(ctx, th.ProjectID)
	if err != nil {
		return err
	}
	if err := svc.store.DeleteThread(ctx, id); err != nil {
		return err
	}
	return removeHistory(proj.WorkDir, th.HistoryPath)
}

func (svc *ThreadService) History(ctx context.Context, id string) ([]map[string]any, error) {
	th, err := svc.store.GetThread(ctx, id)
	if err != nil {
		return nil, err
	}
	proj, err := svc.store.GetProject(ctx, th.ProjectID)
	if err != nil {
		return nil, err
	}
	sb, err := artifactfs.NewSandbox(proj.WorkDir)
	if err != nil {
		return nil, err
	}
	raw, err := sb.Read(th.HistoryPath)
	if os.IsNotExist(err) {
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// 跳过损坏行：容错原则优先于强一致（UX §5.3）。
			continue
		}
		out = append(out, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// 按 seq 升序显式排序：防止 append 顺序被外部工具破坏后前端时间线错乱。
	sort.SliceStable(out, func(i, j int) bool {
		return historySeq(out[i]) < historySeq(out[j])
	})
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

func historySeq(m map[string]any) float64 {
	if v, ok := m["seq"].(float64); ok {
		return v
	}
	return 0
}

func writeEmptyHistory(workDir, rel string) error {
	sb, err := artifactfs.NewSandbox(workDir)
	if err != nil {
		return err
	}
	return sb.Write(rel, []byte{})
}

func removeHistory(workDir, rel string) error {
	sb, err := artifactfs.NewSandbox(workDir)
	if err != nil {
		return err
	}
	if err := sb.Delete(rel); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
