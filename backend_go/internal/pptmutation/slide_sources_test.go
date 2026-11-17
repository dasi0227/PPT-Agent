package pptmutation

import (
	"errors"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"testing"
)

type failAfterWrite struct {
	Workspace
	fail bool
}

func (w *failAfterWrite) Write(path string, raw []byte) error {
	if err := w.Workspace.Write(path, raw); err != nil {
		return err
	}
	if w.fail {
		w.fail = false
		return errors.New("durability failure after replacement")
	}
	return nil
}

func TestBufferRestoresFilesAfterPartialWriteAndMetadataFailure(t *testing.T) {
	_, workspace := mutationFixture(t)
	workspace[model.SlideSpecPath("sli_a")] = []byte(`{"purpose":"content_explanation","layout":"Title above content","core":"Before","elements":[]}`)
	before := string(workspace[model.SlideSpecPath("sli_a")])
	flaky := &failAfterWrite{Workspace: workspace, fail: true}
	buffer := NewBuffer(flaky)
	_ = buffer.Write(model.SlideSpecPath("sli_a"), []byte(`{"changed":true}`))
	if err := buffer.Commit(); err == nil {
		t.Fatal("expected write failure")
	}
	if string(workspace[model.SlideSpecPath("sli_a")]) != before {
		t.Fatal("partial replacement was not rolled back")
	}
	workspace["sli_a.html"] = []byte("old HTML")
	buffer = NewBuffer(workspace)
	_ = buffer.Write(model.SlideSpecPath("sli_a"), []byte(`{}`))
	_ = buffer.Delete("sli_a.html")
	_ = buffer.Write("sli_b.html", []byte("new HTML"))
	if err := buffer.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := buffer.Rollback(); err != nil {
		t.Fatal(err)
	}
	if string(workspace[model.SlideSpecPath("sli_a")]) != before || string(workspace["sli_a.html"]) != "old HTML" {
		t.Fatal("metadata rollback lost existing files")
	}
	if _, exists := workspace["sli_b.html"]; exists {
		t.Fatal("metadata rollback retained new file")
	}
}
