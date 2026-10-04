package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func diffSource(content string) reviewSourceFile {
	return reviewSourceFile{Content: content, Hash: hashBytes([]byte(content))}
}
func TestFinalDiffIsNetAndFrozenAcrossResume(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("sli_1.html", "old\n")
	if err := ensureReviewBaseline(context.Background(), root, "run", false); err != nil {
		t.Fatal(err)
	}
	write("sli_1.html", "intermediate\n")
	if err := ensureReviewBaseline(context.Background(), root, "run", true); err != nil {
		t.Fatal(err)
	}
	write("sli_1.html", "final\n")
	state := &RunState{runID: "run", projectDir: root}
	targets := finalAffectedTargets(state)
	if len(targets) != 1 || targets[0].Insertions != 1 || targets[0].Deletions != 1 {
		t.Fatalf("targets=%+v", targets)
	}
	payload := model.MessageFinalPayload{PublicEventBase: publicBase("run"), MessageID: "m", Text: "完成", AffectedTargets: targets}
	if err := model.ValidatePublicEvent(model.EventMessageFinal, payload); err != nil {
		t.Fatal(err)
	}
	saved, _ := json.Marshal(payload)
	write("sli_1.html", "another run\n")
	current, _ := json.Marshal(payload)
	if string(saved) != string(current) {
		t.Fatal("terminal projection changed with current files")
	}
	write("sli_1.html", "old\n")
	if got := finalAffectedTargets(state); len(got) != 0 {
		t.Fatalf("reverted changes remain: %+v", got)
	}
}
func TestFinalDiffCountsFieldsAndArrayEntriesAndSplitsSpecs(t *testing.T) {
	before := map[string]reviewSourceFile{
		".manifest.json": diffSource(`{"title":"old","requirements":["keep","remove","change","duplicate","duplicate"]}`),
		".spec.json":     diffSource(`{"sli_a":{"core":"old","elements":[],"layout":"old"},"sli_b":{"core":"keep","elements":[]}}`),
	}
	after := map[string]reviewSourceFile{
		".manifest.json": diffSource(`{"title":"new","requirements":["keep","add","changed","duplicate"]}`),
		".spec.json":     diffSource("{\n\"sli_b\":{\"elements\":[],\"core\":\"keep\"},\"sli_a\":{\"core\":\"new\",\"elements\":[],\"purpose\":\"conclusion\"}}"),
	}
	targets := sourceDiffTargets(t.TempDir(), before, after)
	if len(targets) != 2 {
		t.Fatalf("targets=%+v", targets)
	}
	for _, target := range targets {
		if err := target.Diff.Validate(); err != nil {
			t.Fatal(err)
		}
		if target.Part == "manifest" {
			if target.Insertions != 3 || target.Deletions != 4 {
				t.Fatalf("counts=%+v", target)
			}
			for _, field := range target.Diff.Fields {
				added := false
				for _, row := range field.Rows {
					if row.Kind == "added" {
						added = true
					}
					if row.Kind == "removed" && added {
						t.Fatal("old values must precede new values")
					}
				}
			}
		} else if target.SlideID != "sli_a" || target.Insertions != 2 || target.Deletions != 2 {
			t.Fatalf("spec isolation=%+v", target)
		}
	}
}
func TestFinalDiffHTMLRangesAndDeletedSource(t *testing.T) {
	old := "<main>\n  old\n</main>\n"
	next := "<main>\n  new\n  added\n</main>\n"
	hunks, added, removed := textDiff(old, next)
	if added != 2 || removed != 1 || len(hunks) != 1 {
		t.Fatalf("diff=%+v %d %d", hunks, added, removed)
	}
	for _, row := range hunks[0].Rows {
		if row.Kind == "added" && row.OldLine != 0 {
			t.Fatal("added row has an old line")
		}
		if row.Kind == "removed" && row.NewLine != 0 {
			t.Fatal("removed row has a new line")
		}
	}
	targets := sourceDiffTargets(t.TempDir(), map[string]reviewSourceFile{"sli_a.html": diffSource(old)}, map[string]reviewSourceFile{})
	if len(targets) != 1 || targets[0].Diff.Status != "deleted" || targets[0].Deletions != 3 {
		t.Fatalf("deleted source=%+v", targets)
	}
	if rows, a, d := textDiff("", "x\n"); len(rows) != 1 || a != 1 || d != 0 {
		t.Fatal("new file includes a phantom newline")
	}
}
func TestFinalDiffJSONFormattingAndFileIdentity(t *testing.T) {
	before := map[string]reviewSourceFile{".design.json": diffSource(`{"demands":["same"],"decorations":{"page_number":"bottom-right"}}`), "a.txt": diffSource("old\n"), "b.txt": diffSource("old\n")}
	after := map[string]reviewSourceFile{".design.json": diffSource("{\n  \"decorations\": {\"page_number\":\"bottom-right\"},\n  \"demands\": [\"same\"]\n}\n"), "a.txt": diffSource("a\n"), "b.txt": diffSource("b\n")}
	targets := sourceDiffTargets(t.TempDir(), before, after)
	if len(targets) != 2 || targets[0].Diff.Filename == targets[1].Diff.Filename {
		t.Fatalf("format-only change or merged files: %+v", targets)
	}
	for _, target := range targets {
		raw, _ := json.Marshal(target.Diff)
		if strings.Contains(string(raw), `"path"`) {
			t.Fatal("public projection uses forbidden internal path field")
		}
	}
}

func TestFinalDiffDistinguishesNullAndAbsence(t *testing.T) {
	before := map[string]reviewSourceFile{"data.json": diffSource(`{"removed":null,"changed":null}`)}
	after := map[string]reviewSourceFile{"data.json": diffSource(`{"added":null,"changed":"text"}`)}
	targets := sourceDiffTargets(t.TempDir(), before, after)
	if len(targets) != 1 || targets[0].Insertions != 2 || targets[0].Deletions != 2 {
		t.Fatalf("null fields were conflated with absent fields: %+v", targets)
	}
}

func TestFinalDiffOutlineInsertDoesNotMoveExistingSiblings(t *testing.T) {
	old := `{"sections":[{"id":"sec_a","title":"A","purpose":"P","slides":[{"slide_id":"sli_a","title":"A"},{"slide_id":"sli_b","title":"B"}],"subsections":[]}]}`
	next := `{"sections":[{"id":"sec_a","title":"A","purpose":"P","slides":[{"slide_id":"sli_new","title":"New"},{"slide_id":"sli_a","title":"A"},{"slide_id":"sli_b","title":"B"}],"subsections":[]}]}`
	targets := sourceDiffTargets(t.TempDir(), map[string]reviewSourceFile{".outline.json": diffSource(old)}, map[string]reviewSourceFile{".outline.json": diffSource(next)})
	if len(targets) != 1 || targets[0].Insertions != 1 || targets[0].Deletions != 0 {
		t.Fatalf("insertion moved unchanged siblings: %+v", targets)
	}
	if targets[0].Diff.Kind != "outline" || len(targets[0].Diff.Groups) != 1 {
		t.Fatalf("outline was not projected as a frozen tree: %+v", targets[0].Diff)
	}
}

func TestOutlineTreeSubsectionsAndInheritedMoves(t *testing.T) {
	old := `{"sections":[{"id":"sec_a","title":"A","purpose":"P","slides":[],"subsections":[{"id":"sub_a","title":"Sub","purpose":"S","slides":[{"slide_id":"sli_a","title":"One"},{"slide_id":"sli_b","title":"Two"}]}]},{"id":"sec_b","title":"B","purpose":"Q","slides":[],"subsections":[]}]}`
	cases := []struct {
		name, next     string
		added, removed int
	}{
		{"move chapter", `{"sections":[{"id":"sec_b","title":"B","purpose":"Q","slides":[],"subsections":[]},{"id":"sec_a","title":"A","purpose":"P","slides":[],"subsections":[{"id":"sub_a","title":"Sub","purpose":"S","slides":[{"slide_id":"sli_a","title":"One"},{"slide_id":"sli_b","title":"Two"}]}]}]}`, 1, 1},
		{"move subsection", `{"sections":[{"id":"sec_a","title":"A","purpose":"P","slides":[],"subsections":[]},{"id":"sec_b","title":"B","purpose":"Q","slides":[],"subsections":[{"id":"sub_a","title":"Sub","purpose":"S","slides":[{"slide_id":"sli_a","title":"One"},{"slide_id":"sli_b","title":"Two"}]}]}]}`, 1, 1},
		{"delete subtree", `{"sections":[{"id":"sec_a","title":"A","purpose":"P","slides":[],"subsections":[]},{"id":"sec_b","title":"B","purpose":"Q","slides":[],"subsections":[]}]}`, 0, 3},
		{"rename page and purpose", `{"sections":[{"id":"sec_a","title":"A","purpose":"P","slides":[],"subsections":[{"id":"sub_a","title":"Sub","purpose":"Updated","slides":[{"slide_id":"sli_a","title":"New one"},{"slide_id":"sli_b","title":"Two"}]}]},{"id":"sec_b","title":"B","purpose":"Q","slides":[],"subsections":[]}]}`, 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targets := sourceDiffTargets(t.TempDir(), map[string]reviewSourceFile{".outline.json": diffSource(old)}, map[string]reviewSourceFile{".outline.json": diffSource(tc.next)})
			if len(targets) != 1 || targets[0].Insertions != tc.added || targets[0].Deletions != tc.removed {
				t.Fatalf("wrong tree counts: %+v", targets)
			}
			if err := targets[0].Diff.Validate(); err != nil {
				t.Fatal(err)
			}
			for _, group := range targets[0].Diff.Groups {
				for _, row := range group.Rows {
					if row.Node == "page" && row.Depth != 2 {
						t.Fatalf("lost subsection hierarchy: %+v", row)
					}
					if strings.HasPrefix(tc.name, "move") && row.Node == "page" && row.Kind != "context" {
						t.Fatalf("moving an ancestor changed its unchanged page: %+v", row)
					}
				}
			}
		})
	}
	// Adding a whole subsection paints and counts its descendants too.
	targets := sourceDiffTargets(t.TempDir(), map[string]reviewSourceFile{".outline.json": diffSource(cases[2].next)}, map[string]reviewSourceFile{".outline.json": diffSource(old)})
	if len(targets) != 1 || targets[0].Insertions != 3 || targets[0].Deletions != 0 {
		t.Fatalf("wrong added subtree: %+v", targets)
	}
}

func TestOperationDiffUsesItsOwnPreimageAndSurvivesReceiptReplay(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(".outline.json", `{"sections":[{"id":"sec_a","title":"A","purpose":"P","slides":[{"slide_id":"sli_a","title":"A"}],"subsections":[]}]}`)
	write("sli_a.html", "original\n")
	session, err := NewRunSession(root, "operation")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	if _, err = session.Write(slideHTMLRef("sli_a"), "edit_html", []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	// A concurrent unrelated disk change is not part of this tool operation.
	write(".manifest.json", `{"title":"unrelated"}`)
	first := operationDiffTargets(session)
	if len(first) != 1 || first[0].DisplayName != "第 1 页" || first[0].Insertions != 1 || first[0].Deletions != 1 {
		t.Fatalf("operation targets=%+v", first)
	}
	if _, err = session.CommitOperation(context.Background(), "one", "{}", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = session.Write(slideHTMLRef("sli_a"), "edit_html", []byte("second\n")); err != nil {
		t.Fatal(err)
	}
	second := operationDiffTargets(session)
	if len(second) != 1 || second[0].Diff.Hunks[0].Rows[0].Text != "first" || first[0].Diff.Hunks[0].Rows[0].Text != "original" {
		t.Fatal("operation diff reused the run baseline or mutated an earlier projection")
	}
	input := RuntimeInput{ProjectDir: root, Idempotency: newMemoryIdempotencyStore()}
	state := &RunState{runID: "operation", scope: model.NewRunScope(model.ScopeAllPages)}
	call := llm.ToolCall{ID: "one", Name: "run_command", Args: map[string]any{"command": "sed -i substitution sli_a.html"}}
	runtime := NewRuntime(nil)
	if _, acquired := runtime.acquireToolCall(context.Background(), input, state, call); !acquired {
		t.Fatal("could not acquire receipt")
	}
	saved := SuccessfulToolResult("saved")
	saved.OperationTargets = first
	saved.Command = &CommandExecution{Text: "sed -i substitution sli_a.html", Status: "completed"}
	runtime.persistToolCall(context.Background(), input, state, call, saved)
	replayed, execute := runtime.acquireToolCall(context.Background(), input, state, call)
	if execute || len(replayed.OperationTargets) != 1 || replayed.Command == nil || strings.Contains(modelToolObservation(replayed), "original") {
		t.Fatal("receipt lost its projection or leaked it to model observation")
	}
	projected, ok := (ToolPublicProjector{ProjectDir: root}).Completed(state.runID, call.ID, call.Name, call.Args, replayed)
	if !ok || projected.Changes == nil || (*projected.Changes)[0].Diff.Hunks[0].Rows[0].Text != "original" {
		t.Fatal("replay regenerated a diff from newer content")
	}
	if err = model.ValidatePublicEvent(model.EventToolCompleted, projected); err != nil {
		t.Fatal(err)
	}
}
