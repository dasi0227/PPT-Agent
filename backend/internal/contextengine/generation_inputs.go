package contextengine

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func loadHTMLState(workDir, slideID string) string {
	if _, err := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath(slideID)))); err != nil {
		return string(model.HTMLMissing)
	}
	return string(model.HTMLAvailable)
}

func setGenerationInputs(pack *ContextPack, id string, slide spec.SlideSpec) {
	if pack.GenerationInputs == nil {
		pack.GenerationInputs = map[string]*spec.GenerationInputs{}
	}
	delete(pack.GenerationInputs, id)
	value := spec.GenerationInputs{Manifest: pack.PresentationManifest.Manifest, Spec: slide}
	if pack.Design.Design != nil {
		value.Design = *pack.Design.Design
	}
	pack.GenerationInputs[id] = value.Clone()
}

// AcceptGenerationInputs runs only after the file/metadata transaction commits.
// It updates the in-memory baseline without rereading later reference content.
func AcceptGenerationInputs(pack *ContextPack, inputs map[string]json.RawMessage) {
	if pack.GenerationBaselines == nil {
		pack.GenerationBaselines = map[string]*spec.GenerationInputs{}
	}
	for id, raw := range inputs {
		pack.GenerationBaselines[id] = spec.ParseGenerationInputs(raw)
	}
}
