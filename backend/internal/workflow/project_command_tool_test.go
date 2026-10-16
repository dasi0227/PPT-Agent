package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestCommandReadsNeedNoApprovalAndWritesArePreparedOnce(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires macOS Seatbelt")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=value"), 0600)
	session, _ := NewRunSession(dir, "shell")
	defer session.Discard()
	input := DomainToolInput{ProjectDir: dir, Session: session, Mode: model.ModeExecute, Phase: PhaseExecuting}
	tool := projectCommandTool{}
	input.Args = map[string]any{"command": "cat .env"}
	read := tool.Preflight(context.Background(), input)
	if read.Outcome != "allow" || read.Mutates {
		t.Fatalf("read=%+v", read)
	}
	input.Decision = &read
	if got := tool.Execute(context.Background(), input); !got.OK {
		t.Fatalf("read=%+v", got)
	}
	input.Args = map[string]any{"command": "mkdir output; printf hello > output/a; cp output/a output/b; chmod +x output/b; ln -s a output/link"}
	write := tool.Preflight(context.Background(), input)
	if write.Outcome != "confirm" || !write.Mutates {
		t.Fatalf("write=%+v", write)
	}
	if _, err := os.Stat(filepath.Join(dir, "output")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preparation changed live project")
	}
	input.Decision = &write
	result := tool.Execute(context.Background(), input)
	if !result.OK {
		t.Fatalf("write=%+v", result)
	}
	if _, err := session.CommitOperation(context.Background(), "write", "", nil); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "output/link"))
	if string(raw) != "hello" {
		t.Fatalf("link contents=%q", raw)
	}
	info, _ := os.Stat(filepath.Join(dir, "output/b"))
	if info == nil || info.Mode()&0111 == 0 {
		t.Fatal("executable bit lost")
	}
}

func TestCommandPreparationRejectsStaleApproval(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires macOS Seatbelt")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "notes")
	_ = os.WriteFile(path, []byte("old"), 0600)
	session, _ := NewRunSession(dir, "stale")
	defer session.Discard()
	input := DomainToolInput{ProjectDir: dir, Session: session, Mode: model.ModeExecute, Phase: PhaseExecuting, Args: map[string]any{"command": "printf new > notes"}}
	tool := projectCommandTool{}
	decision := tool.Preflight(context.Background(), input)
	input.Decision = &decision
	_ = os.WriteFile(path, []byte("external"), 0600)
	if got := tool.Execute(context.Background(), input); got.Code != model.CodeContentConflict {
		t.Fatalf("stale=%+v", got)
	}
}

func TestCommandNodeRollbackRestoresDirectoriesModesAndLinks(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "old"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "old/script"), []byte("before"), 0755)
	before, err := commandexec.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	session, _ := NewRunSession(dir, "rollback")
	defer session.Discard()
	session.commandBaseline = before.Hash()
	session.stageCommandNode("old/script", before, nil)
	session.stageCommandNode("old", before, nil)
	session.stageCommandNode("new", before, &commandexec.Node{Kind: "directory", Mode: 0755})
	session.stageCommandNode("new/link", before, &commandexec.Node{Kind: "symlink", Mode: 0777, Content: []byte("../old/script")})
	_, err = session.CommitOperation(context.Background(), "rollback", "", func(context.Context, CommitContext) error { return errors.New("metadata failure") })
	if err == nil {
		t.Fatal("expected metadata failure")
	}
	after, err := commandexec.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if after.Hash() != before.Hash() {
		t.Fatal("rollback did not restore the original tree")
	}
}
