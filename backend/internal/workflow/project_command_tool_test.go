package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
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
	if got := tool.Execute(context.Background(), input); got.Code != model.CodeInternal {
		t.Fatal("unapproved candidate executed")
	}
	write.Approved = true
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

func TestCommandCandidateResolvesOutlineAndFreezesCascade(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	before, err := commandexec.Snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	after := maps.Clone(before)
	var outline spec.Outline
	if err := json.Unmarshal(before[".outline.json"].Content, &outline); err != nil {
		t.Fatal(err)
	}
	outline.Sections[0].Slides = []spec.SlideNode{{ID: "new", Title: "New page"}}
	raw, _ := json.Marshal(outline)
	node := after[".outline.json"]
	node.Content = raw
	after[".outline.json"] = node
	if err := normalizeCommandOutline(before, after); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after[".outline.json"].Content, &outline); err != nil {
		t.Fatal(err)
	}
	newID := outline.Sections[0].Slides[0].ID
	if newID == "new" || newID == generationSlide {
		t.Fatal("new identity was not allocated")
	}
	for _, path := range []string{model.SlideSpecPath(generationSlide), model.SlideHTMLPath(generationSlide)} {
		if _, ok := after[path]; ok {
			t.Fatalf("removed page source survived: %s", path)
		}
	}
	p := preparedCommand{Decision: commandexec.Decision{PreimageHash: before.Hash()}, Before: before, Changes: map[string]*commandexec.Node{}}
	for path, next := range after {
		if next.Hash() != before[path].Hash() {
			value := next
			p.Changes[path] = &value
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			p.Changes[path] = nil
		}
	}
	session := p.session(dir)
	session.runID = "candidate"
	pack.Command.Scope = model.NewRunScope(model.ScopeCurrentPage, "sli_unrelated")
	if err := session.ValidateStagedArtifacts(pack); err != nil {
		t.Fatal(err)
	}
	if issues := (ScopeCompletionPolicy{}).Check(CompletionContext{Mode: model.ModeExecute, Scope: pack.Command.Scope, Changes: session.ChangeSet()}); len(issues) != 0 {
		t.Fatal(issues)
	}
	if AllowsArtifact(pack.Command.Scope, slideHTMLRef(newID)) {
		t.Fatal("command approval widened future editing scope")
	}
	targets := operationDiffTargets(session)
	if len(targets) != 3 {
		t.Fatalf("missing cascade diffs: %+v", targets)
	}
	for _, target := range targets {
		if target.Diff.Kind != "text" {
			t.Fatalf("command diff is not source text: %+v", target)
		}
	}
	if _, err := session.CommitOperation(context.Background(), "approved", "", nil); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(filepath.Join(dir, ".outline.json"))
	if string(saved) != string(after[".outline.json"].Content) {
		t.Fatal("commit regenerated the approved outline")
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
	decision.Approved = true
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
