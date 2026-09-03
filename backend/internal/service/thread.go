package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
	steering, steeringErr := svc.store.ListThreadSteering(ctx, id)
	if steeringErr == nil {
		existing := map[string]int{}
		nextSyntheticSeq := float64(1)
		for index, entry := range out {
			if seq, ok := entry["seq"].(float64); ok && seq >= nextSyntheticSeq {
				nextSyntheticSeq = seq + 1
			}
			if entry["type"] == "steering" {
				if data, ok := entry["data"].(map[string]any); ok {
					existing[fmt.Sprint(data["client_message_id"])] = index
				}
			}
		}
		for _, message := range steering {
			if index, ok := existing[message.ClientMessageID]; ok {
				if data, ok := out[index]["data"].(map[string]any); ok {
					data["status"] = message.Status
					data["rejection_code"] = message.RejectionCode
				}
				continue
			}
			out = append(out, map[string]any{
				"seq": nextSyntheticSeq, "ts": message.AcceptedAt / int64(time.Second),
				"run_id": message.RunID, "turn": "user", "type": "steering",
				"data": map[string]any{
					"client_message_id": message.ClientMessageID, "text": message.Content,
					"status": message.Status, "rejection_code": message.RejectionCode,
				},
			})
			nextSyntheticSeq++
		}
	}
	commits, commitErr := svc.store.ListThreadGitCommits(ctx, id)
	if commitErr == nil {
		for _, operation := range commits {
			entry := gitCommitHistoryEntry(operation)
			insertAt := len(out)
			for index, existing := range out {
				if historyTimestamp(existing) > operation.UpdatedAt {
					insertAt = index
					break
				}
			}
			out = append(out, nil)
			copy(out[insertAt+1:], out[insertAt:])
			out[insertAt] = entry
		}
	}
	briefings, briefingErr := svc.store.ListThreadBriefings(ctx, id)
	if briefingErr == nil {
		for _, briefing := range briefings {
			entry := briefingHistoryEntry(briefing)
			insertAt := len(out)
			for index, existing := range out {
				if historyTimestamp(existing) > briefing.UpdatedAt {
					insertAt = index
					break
				}
			}
			out = append(out, nil)
			copy(out[insertAt+1:], out[insertAt:])
			out[insertAt] = entry
		}
	}
	runOrder := map[string]int{}
	for _, entry := range out {
		runID := fmt.Sprint(entry["run_id"])
		if _, exists := runOrder[runID]; !exists {
			runOrder[runID] = len(runOrder)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftRun, rightRun := fmt.Sprint(out[i]["run_id"]), fmt.Sprint(out[j]["run_id"])
		if runOrder[leftRun] != runOrder[rightRun] {
			return runOrder[leftRun] < runOrder[rightRun]
		}
		return historySeq(out[i]) < historySeq(out[j])
	})
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

func gitCommitHistoryEntry(operation model.GitCommitOperation) map[string]any {
	occurredAt := time.Unix(operation.UpdatedAt, 0).UTC().Format(time.RFC3339Nano)
	base := map[string]any{
		"schema_version": model.GitCommitEventSchemaVersion,
		"operation_id":   operation.ID, "project_id": operation.ProjectID,
		"thread_id": operation.ThreadID, "occurred_at": occurredAt,
	}
	entryType := string(model.EventGitCommitFailed)
	if operation.Status == model.GitCommitCompleted {
		entryType = string(model.EventGitCommitCompleted)
		var result any
		if json.Unmarshal([]byte(operation.ResultJSON), &result) == nil {
			base["commit"] = result
		}
	} else {
		var publicError any
		if json.Unmarshal([]byte(operation.ErrorJSON), &publicError) == nil {
			base["error"] = publicError
		}
	}
	return map[string]any{
		"seq": 1, "ts": operation.UpdatedAt, "run_id": operation.ID,
		"turn": "agent", "type": entryType, "data": base,
	}
}

func briefingHistoryEntry(briefing model.Briefing) map[string]any {
	return map[string]any{
		"seq": 1, "ts": briefing.UpdatedAt, "run_id": briefing.BriefingID,
		"turn": "agent", "type": "briefing",
		"data": map[string]any{
			"briefing_id": briefing.BriefingID,
			"project_id":  briefing.ProjectID,
			"thread_id":   briefing.ThreadID,
			"kind":        briefing.Kind,
			"versions":    briefing.Versions,
			"updated_at":  briefing.UpdatedAt,
		},
	}
}

func historyTimestamp(entry map[string]any) int64 {
	switch value := entry["ts"].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}

func historySeq(entry map[string]any) float64 {
	if value, ok := entry["seq"].(float64); ok {
		return value
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
