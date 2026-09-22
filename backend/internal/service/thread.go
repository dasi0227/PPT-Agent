package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// ThreadService 管理一个 project 下的对话线程与 history jsonl。
type ThreadService struct {
	store       store.Store
	clock       func() int64
	newID       func() string
	transcripts *contextengine.FSTranscriptStore
}

type CreateThreadParams struct {
	Title string
}

func NewThreadService(s store.Store) *ThreadService {
	return NewThreadServiceWithTranscript(s, contextengine.NewFSTranscriptStore())
}

func NewThreadServiceWithTranscript(s store.Store, transcripts *contextengine.FSTranscriptStore) *ThreadService {
	if transcripts == nil {
		transcripts = contextengine.NewFSTranscriptStore()
	}
	return &ThreadService{
		store: s, clock: func() int64 { return time.Now().Unix() }, newID: uuid.NewString,
		transcripts: transcripts,
	}
}

func (svc *ThreadService) CreateThread(ctx context.Context, projectID string, p CreateThreadParams) (model.Thread, error) {
	proj, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return model.Thread{}, err
	}
	id := svc.newID()
	now := svc.clock()
	title := strings.TrimSpace(p.Title)
	if title != "" {
		title, err = ValidateThreadTitle(title)
		if err != nil {
			return model.Thread{}, err
		}
	}
	th := model.Thread{
		ID:                     id,
		ProjectID:              projectID,
		Title:                  title,
		HistoryPath:            model.UserHistoryPath(id),
		Status:                 "active",
		AutoRenameEnabled:      title == "",
		NamingRevision:         1,
		RenameOperationVersion: 1,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := writeEmptyHistory(proj.WorkDir, th.HistoryPath); err != nil {
		return model.Thread{}, err
	}
	if err := svc.transcripts.Create(proj.WorkDir, th.ID); err != nil {
		_ = removeHistory(proj.WorkDir, th.HistoryPath)
		return model.Thread{}, err
	}
	if err := svc.store.CreateThread(ctx, th); err != nil {
		_ = removeHistory(proj.WorkDir, th.HistoryPath)
		_ = svc.transcripts.Remove(proj.WorkDir, th.ID)
		return model.Thread{}, err
	}
	return th, nil
}

func (svc *ThreadService) RenameThread(ctx context.Context, id, title string) (model.Thread, error) {
	clean, err := ValidateThreadTitle(title)
	if err != nil {
		return model.Thread{}, err
	}
	enabled := false
	return svc.store.UpdateThreadNamingState(ctx, id, &clean, &enabled, true, svc.clock())
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
	if err := removeHistory(proj.WorkDir, th.HistoryPath); err != nil {
		return err
	}
	return svc.transcripts.Remove(proj.WorkDir, th.ID)
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
	sb, err := artifactfs.NewSandbox(model.ProjectRoot(proj.WorkDir))
	if err != nil {
		return nil, err
	}
	raw, err := sb.Read(th.HistoryPath)
	if os.IsNotExist(err) {
		raw, err = nil, nil
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
	// JSONL is a projection: recover any missing entry from the durable event log.
	events, eventErr := svc.store.ListThreadEvents(ctx, id)
	if eventErr != nil {
		return nil, eventErr
	}
	entryIndex := map[string]int{}
	for index, entry := range out {
		// Steering rows share a sequence with the next public run event.
		if entry["type"] != "steering" {
			entryIndex[fmt.Sprintf("%v:%d", entry["run_id"], int64(historySeq(entry)))] = index
		}
	}
	for _, event := range events {
		entry, visible := run.PublicHistoryEntry(event)
		if !visible {
			continue
		}
		data := map[string]any{"seq": entry.Seq, "ts": entry.TS, "run_id": entry.RunID, "turn": entry.Turn, "type": entry.Type, "data": entry.Data}
		key := fmt.Sprintf("%s:%d", entry.RunID, entry.Seq)
		if index, exists := entryIndex[key]; exists {
			out[index] = data
		} else {
			entryIndex[key] = len(out)
			out = append(out, data)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return historyTimestamp(out[i]) < historyTimestamp(out[j]) })
	steering, steeringErr := svc.store.ListThreadSteering(ctx, id)
	if steeringErr == nil {
		existing := map[string]int{}
		nextSyntheticSeq := float64(1)
		for index, entry := range out {
			if seq := historySeq(entry); seq >= nextSyntheticSeq {
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
					data["attachments"] = message.Attachments
					data["dom_selections"] = model.PublicDOMSelections(message.DOMSelections)
					data["reference_order"] = message.ReferenceOrder
				}
				continue
			}
			out = append(out, map[string]any{
				"seq": nextSyntheticSeq, "ts": message.AcceptedAt / int64(time.Second),
				"run_id": message.RunID, "turn": "user", "type": "steering",
				"data": map[string]any{
					"client_message_id": message.ClientMessageID, "text": message.Content,
					"attachments": message.Attachments, "dom_selections": model.PublicDOMSelections(message.DOMSelections),
					"reference_order": message.ReferenceOrder, "status": message.Status, "rejection_code": message.RejectionCode,
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
	activities, activityErr := svc.store.ListThreadCommandActivities(ctx, id)
	if activityErr != nil {
		return nil, activityErr
	}
	compactions, compactionErr := listThreadContextCompactions(ctx, svc.store, id)
	if compactionErr != nil {
		return nil, compactionErr
	}
	completedCompactions := make(map[string]bool, len(compactions))
	for _, compaction := range compactions {
		completedCompactions["context-compaction:"+compaction.ID] = true
	}
	activityIDs := make(map[string]bool, len(activities))
	for _, activity := range activities {
		activityIDs[activity.ID] = true
	}
	for _, activity := range interruptedCompactionActivities(events, th) {
		if !activityIDs[activity.ID] && !completedCompactions[activity.ID] {
			activities = append(activities, activity)
		}
	}
	briefingCommands, compactCommands := map[string]bool{}, map[string]bool{}
	for _, activity := range activities {
		var request struct {
			BriefingID string `json:"briefing_id"`
		}
		var result struct {
			Briefing struct {
				ID string `json:"briefing_id"`
			} `json:"briefing"`
			Compaction struct {
				ID string `json:"id"`
			} `json:"compaction"`
		}
		_ = json.Unmarshal(activity.Request, &request)
		_ = json.Unmarshal(activity.Result, &result)
		if request.BriefingID != "" {
			briefingCommands[request.BriefingID] = true
		}
		if result.Briefing.ID != "" {
			briefingCommands[result.Briefing.ID] = true
		}
		if result.Compaction.ID != "" {
			compactCommands[result.Compaction.ID] = true
		}
		entry := map[string]any{"seq": 1, "ts": activity.CreatedAt / 1000, "run_id": activity.ID, "turn": "agent", "type": "command_activity", "data": activity}
		insertAt := len(out)
		for index, existing := range out {
			if historyTimestamp(existing) > activity.CreatedAt/1000 {
				insertAt = index
				break
			}
		}
		out = append(out, nil)
		copy(out[insertAt+1:], out[insertAt:])
		out[insertAt] = entry
	}
	briefings, briefingErr := svc.store.ListThreadBriefings(ctx, id)
	if briefingErr == nil {
		for _, briefing := range briefings {
			if briefingCommands[briefing.BriefingID] {
				continue
			}
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
	for _, compaction := range compactions {
		if compactCommands[compaction.ID] {
			continue
		}
		entry := contextCompactionHistoryEntry(compaction)
		insertAt := len(out)
		for index, existing := range out {
			if historyTimestamp(existing) > compaction.CreatedAt {
				insertAt = index
				break
			}
		}
		out = append(out, nil)
		copy(out[insertAt+1:], out[insertAt:])
		out[insertAt] = entry
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
	if operation.Status == model.GitCommitAccepted || operation.Status == model.GitCommitRunning || operation.Status == model.GitCommitEmpty {
		base["status"] = operation.Status
		return map[string]any{"seq": 1, "ts": operation.CreatedAt, "run_id": operation.ID, "turn": "agent", "type": "git.commit.state", "data": base}
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

type contextCompactionReader interface {
	ListThreadContextCompactions(context.Context, string) ([]model.ContextCompaction, error)
}

func listThreadContextCompactions(
	ctx context.Context,
	value any,
	threadID string,
) ([]model.ContextCompaction, error) {
	reader, ok := value.(contextCompactionReader)
	if !ok {
		return []model.ContextCompaction{}, nil
	}
	return reader.ListThreadContextCompactions(ctx, threadID)
}

func contextCompactionHistoryEntry(compaction model.ContextCompaction) map[string]any {
	return map[string]any{
		"seq": 1, "ts": compaction.CreatedAt, "run_id": compaction.ID,
		"turn": "agent", "type": "context_compaction",
		"data": map[string]any{
			"id": compaction.ID, "thread_id": compaction.ThreadID,
			"project_id": compaction.ProjectID, "run_id": compaction.RunID,
			"trigger": compaction.Trigger, "title": compaction.Title, "content": compaction.Content,
			"before_tokens": compaction.BeforeTokens, "after_tokens": compaction.AfterTokens,
			"max_tokens": compaction.MaxTokens, "reclaimed_tokens": compaction.Reclaimed,
			"duration_ms": compaction.DurationMS, "created_at": compaction.CreatedAt,
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
	switch value := entry["seq"].(type) {
	case float64:
		return value
	case int64:
		return float64(value)
	case int:
		return float64(value)
	}
	return 0
}

func writeEmptyHistory(workDir, rel string) error {
	sb, err := artifactfs.NewSandbox(model.ProjectRoot(workDir))
	if err != nil {
		return err
	}
	return sb.Write(rel, []byte{})
}

func removeHistory(workDir, rel string) error {
	sb, err := artifactfs.NewSandbox(model.ProjectRoot(workDir))
	if err != nil {
		return err
	}
	if err := sb.Delete(rel); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
