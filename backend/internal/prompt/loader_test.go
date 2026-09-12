package prompt

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	embedded "github.com/dasi0227/PPT-Agent/backend"
)

func TestCatalogMatchesEmbeddedMarkdown(t *testing.T) {
	paths := map[string]bool{}
	for id, e := range catalog {
		if paths[e.Path] {
			t.Fatalf("duplicate catalog path %s", e.Path)
		}
		paths[e.Path] = true
		m, err := Load(id)
		if err != nil {
			t.Fatal(err)
		}
		if m.Version == "" || m.ID != id || m.Path != e.Path || m.Hash != fmt.Sprintf("%x", sha256.Sum256([]byte(m.Body))) {
			t.Fatalf("invalid module metadata: %+v", m)
		}
	}
	err := fs.WalkDir(embedded.PromptFiles, "prompts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") || !paths[path] {
			t.Errorf("unexpected/unregistered embedded file %s", path)
		}
		delete(paths, path)
		return nil
	})
	if err != nil || len(paths) != 0 {
		t.Fatalf("unmatched catalog: %v, %v", paths, err)
	}
}

func TestLoadRejectsUnknownMissingAndEmpty(t *testing.T) {
	if _, err := Load("default"); err == nil {
		t.Fatal("accepted unknown ID")
	}
	if _, err := load(fstest.MapFS{}, "core.agent"); err == nil {
		t.Fatal("accepted missing file")
	}
	if _, err := load(fstest.MapFS{"prompts/core/agent.md": &fstest.MapFile{Data: []byte(" \n ")}}, "core.agent"); err == nil {
		t.Fatal("accepted empty file")
	}
}

func TestRenderedBodyRefreshesHash(t *testing.T) {
	source := MustLoad("core.structure")
	rendered, err := source.WithBody(strings.ReplaceAll(source.Body, "{{CONTRACTS_JSON}}", "{}"))
	if err != nil || rendered.Hash == source.Hash || rendered.Path != source.Path || rendered.Version != source.Version {
		t.Fatalf("render metadata: %+v, %v", rendered, err)
	}
	if _, err := source.WithBody("\n"); err == nil {
		t.Fatal("accepted empty rendered body")
	}
}
