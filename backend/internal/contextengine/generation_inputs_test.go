package contextengine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestGenerationBaselinesRemainInternalAndDoNotAdvanceOnReads(t *testing.T) {
	project, store := fixture(t)
	assembler := NewContextAssembler(store)
	request := ContextRequest{RunID: "run", ThreadID: "thread", ProjectID: project.ID, Command: testScopeCommand(model.ScopeAllPages)}
	initial, err := assembler.Assemble(context.Background(), request, project)
	if err != nil {
		t.Fatal(err)
	}
	for id, value := range initial.GenerationInputs {
		old := value.Clone()
		old.Design.Demands = []string{"previous " + id}
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
	for id, baseline := range pack.GenerationBaselines {
		change := spec.DiffGenerationInputs(baseline, pack.GenerationInputs[id])
		if change == nil || string(change.Design["/demands"].Old) != `["previous `+id+`"]` {
			t.Fatal("baseline lost")
		}
	}

	request.Command = testScopeCommand(model.ScopeCurrentPage)
	request.Command.MentionedPages = []model.MentionedPage{{SlideID: "sli_aaaaaa", Kind: "slide"}}
	pack = assemble()

	// Reads/assembly never advance the baseline; missing HTML has no differences.
	if err := os.Remove(filepath.Join(project.WorkDir, model.SlideHTMLPath("sli_aaaaaa"))); err != nil {
		t.Fatal(err)
	}
	meta := store.slides["sli_bbbbbb"]
	invalid := `{"spec":{}}`
	meta.GenerationInputsJSON = &invalid
	store.slides[meta.ID] = meta
	pack = assemble()
	if pack.GenerationBaselines["sli_bbbbbb"] != nil {
		t.Fatal("invalid baseline accepted")
	}

	if initial.Manifest.PackHash == pack.Manifest.PackHash {
		t.Fatal("changes missing from context identity")
	}
}
