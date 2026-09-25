package contextengine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestReferenceChangesUseEachPageBaselineAndTaskScope(t *testing.T) {
	project, store := fixture(t)
	assembler := testAssembler(store, nil)
	request := ContextRequest{RunID: "run", ThreadID: "thread", ProjectID: project.ID, Command: testScopeCommand(model.ScopeAllPages), Budget: DefaultBudget()}
	initial, err := assembler.Assemble(context.Background(), request, project)
	if err != nil {
		t.Fatal(err)
	}
	for id, value := range initial.GenerationInputs {
		old := value.Clone()
		old.Design.Direction = "previous " + id
		raw, _ := json.Marshal(old)
		text := string(raw)
		meta := store.slides[id]
		meta.GenerationInputsJSON = &text
		store.slides[id] = meta
	}
	assemble := func() ContextPack {
		t.Helper()
		pack, err := assembler.Assemble(context.Background(), request, project)
		if err != nil {
			t.Fatal(err)
		}
		return pack
	}
	pack := assemble()
	changes := referenceChanges(pack)
	if len(changes) != 3 {
		t.Fatalf("all pages: %+v", changes)
	}
	for id, change := range changes {
		if string(change.Design["/direction"].Old) != `"previous `+id+`"` {
			t.Fatalf("shared baseline: %+v", changes)
		}
	}
	request.Command = testScopeCommand(model.ScopeCurrentPage)
	request.Command.MentionedPages = []model.MentionedPage{{SlideID: "sli_aaaaaa", Kind: "slide"}}
	pack = assemble()
	if got := referenceChanges(pack); len(got) != 2 || got["sli_cccccc"] != nil {
		t.Fatalf("scope + references: %+v", got)
	}
	// Reads/assembly never advance the baseline; missing HTML has no differences.
	if err := os.Remove(filepath.Join(project.WorkDir, model.SlideHTMLPath("sli_aaaaaa"))); err != nil {
		t.Fatal(err)
	}
	meta := store.slides["sli_bbbbbb"]
	invalid := `{"spec":{}}`
	meta.GenerationInputsJSON = &invalid
	store.slides[meta.ID] = meta
	pack = assemble()
	changes = referenceChanges(pack)
	if len(changes) != 1 || changes["sli_bbbbbb"].Baseline != "unknown" {
		t.Fatalf("unknown/missing: %+v", changes)
	}
	if initial.Manifest.PackHash == pack.Manifest.PackHash {
		t.Fatal("changes missing from context identity")
	}
}
