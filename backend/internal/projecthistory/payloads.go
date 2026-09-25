package projecthistory

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// CollectPayloads runs at startup, under the workspace writer lock, after outbox
// recovery and before workers start. Live logs and every retained snapshot pin
// payloads; unreferenced files from rolled-back transactions can then be removed.
func (m *Manager) CollectPayloads(ctx context.Context, project model.Project) error {
	state, err := m.State(project.ID)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(ref string) error {
		if ref == "" || visited[ref] {
			return nil
		}
		visited[ref] = true
		snap, err := m.load(project.ID, ref)
		if err != nil {
			return err
		}
		for name := range snap.Files {
			parts := strings.Split(name, "/")
			if len(parts) == 4 && parts[0] == "threads" && parts[2] == "payloads" {
				keep[parts[1]+"/"+parts[3]] = true
			}
		}
		for _, cp := range snap.State.Checkpoints {
			if err := visit(cp.Snapshot); err != nil {
				return err
			}
		}
		return visit(snap.State.Latest)
	}
	for _, cp := range state.Checkpoints {
		if err := visit(cp.Snapshot); err != nil {
			return err
		}
	}
	if err := visit(state.Latest); err != nil {
		return err
	}
	threads, err := m.Store.ListThreads(ctx, project.ID)
	if err != nil {
		return err
	}
	for _, thread := range threads {
		events, err := m.Store.ThreadEvents(ctx, thread.ID, 0)
		if err != nil {
			return err
		}
		for _, event := range events {
			if event.PayloadRef != "" {
				keep[thread.ID+"/"+event.PayloadRef+".json"] = true
			}
			for _, ref := range event.PayloadFields {
				keep[thread.ID+"/"+ref+".json"] = true
			}
		}
		dir := filepath.Join(model.ProjectRoot(project.WorkDir), "threads", thread.ID, "payloads")
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			ref := strings.TrimSuffix(entry.Name(), ".json")
			decoded, err := hex.DecodeString(ref)
			if err != nil || len(decoded) != 32 || entry.IsDir() || keep[thread.ID+"/"+entry.Name()] {
				continue
			}
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
