package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestRestoredDOMChecksCurrentHTMLBeforeResend(t *testing.T) {
	p := model.Project{WorkDir: t.TempDir()}
	path := filepath.Join(p.WorkDir, model.SlideHTMLPath("sli_one"))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("<h1>original</h1>")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	selection := model.DOMSelection{SlideID: "sli_one", HTMLHash: spec.ContentHash(raw)}
	snapshot := spec.ProjectContentSnapshot{SlidesByID: map[string]spec.SlideContent{"sli_one": {HTMLHash: selection.HTMLHash}}}
	if err := validateRestoredDOM(p, snapshot, []model.DOMSelection{selection}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("<h1>changed</h1>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateRestoredDOM(p, snapshot, []model.DOMSelection{selection}); err == nil {
		t.Fatal("changed bytes accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := validateRestoredDOM(p, snapshot, []model.DOMSelection{selection}); err == nil {
		t.Fatal("missing artifact accepted")
	}
}
