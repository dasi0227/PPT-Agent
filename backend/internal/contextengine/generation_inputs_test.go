package contextengine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
		if baseline == nil || len(baseline.Design.Demands) != 1 || baseline.Design.Demands[0] != "previous "+id {
			t.Fatal("baseline lost")
		}
	}

	// Reads/assembly never advance the baseline, even after HTML is removed.
	if err := os.Remove(filepath.Join(project.WorkDir, model.SlideHTMLPath("sli_aaaaaa"))); err != nil {
		t.Fatal(err)
	}
	meta := store.slides["sli_bbbbbb"]
	invalid := `{"spec":{}}`
	meta.GenerationInputsJSON = &invalid
	store.slides[meta.ID] = meta
	pack = assemble()
	if baseline := pack.GenerationBaselines["sli_aaaaaa"]; baseline == nil || baseline.Design.Demands[0] != "previous sli_aaaaaa" {
		t.Fatal("read advanced the baseline")
	}
	if pack.GenerationBaselines["sli_bbbbbb"] != nil {
		t.Fatal("invalid baseline accepted")
	}

	if initial.Manifest.PackHash == pack.Manifest.PackHash {
		t.Fatal("changes missing from context identity")
	}
}
