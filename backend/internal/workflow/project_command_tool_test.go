package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestProjectCommandPreflightClassifiesReadsAndWrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := projectCommandTool{}
	read := tool.Preflight(context.Background(), DomainToolInput{
		ProjectDir: dir, Mode: model.ModeChat, Phase: PhaseChat,
		Args: map[string]any{"command": "cat notes.txt"},
	})
	if read.Outcome != "allow" || read.Mutates {
		t.Fatalf("read=%+v", read)
	}
	write := tool.Preflight(context.Background(), DomainToolInput{
		ProjectDir: dir, Mode: model.ModeExecute, Phase: PhaseExecuting,
		Args: map[string]any{"command": `sed -i '' 's/old/new/g' notes.txt`},
	})
	if write.Outcome != "confirm" || !write.Mutates || write.PreimageHash == "" {
		t.Fatalf("write=%+v", write)
	}
}

func TestProjectCommandEditStagesUntilCommit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	tool := projectCommandTool{}
	args := map[string]any{"command": `sed -i '' 's/old/new/g' notes.txt`}
	decision := tool.Preflight(context.Background(), DomainToolInput{
		ProjectDir: dir, Session: session, Mode: model.ModeExecute, Phase: PhaseExecuting, Args: args,
	})
	result := tool.Execute(context.Background(), DomainToolInput{
		ProjectDir: dir, Session: session, Mode: model.ModeExecute, Phase: PhaseExecuting,
		Args: args, Decision: &decision,
	})
	if !result.OK || len(result.ChangedTargets) != 1 {
		t.Fatalf("result=%+v", result)
	}
	baseline, _ := os.ReadFile(path)
	if string(baseline) != "old\n" {
		t.Fatalf("baseline changed before commit: %q", baseline)
	}
	staged, err := session.Read(projectFileRef("notes.txt"))
	if err != nil || string(staged) != "new\n" {
		t.Fatalf("staged=%q err=%v", staged, err)
	}
	if err := session.Commit(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	committed, _ := os.ReadFile(path)
	if string(committed) != "new\n" {
		t.Fatalf("committed=%q", committed)
	}
}

func TestProjectCommandEditDiscardLeavesBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	session, _ := NewRunSession(dir, "run-1")
	_, err := session.Write(projectFileRef("notes.txt"), "run_command", []byte("new\n"))
	if err != nil {
		t.Fatal(err)
	}
	session.Discard()
	raw, _ := os.ReadFile(path)
	if string(raw) != "old\n" {
		t.Fatalf("discard changed baseline: %q", raw)
	}
}
