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

func TestCatalogMatchesEmbeddedPromptFiles(t *testing.T) {
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
		if strings.HasPrefix(id, "command.") || strings.HasPrefix(id, "tool.") || strings.HasPrefix(id, "jev.") || strings.HasPrefix(id, "decision.") {
			raw, err := fs.ReadFile(embedded.PromptFiles, e.Path)
			if err != nil {
				t.Fatal(err)
			}
			if m.Description != "" || m.Body != strings.TrimSpace(string(raw)) {
				t.Fatalf("prompt %s was not loaded as plain text", id)
			}
		}
		if strings.HasPrefix(id, "tool.") {
			if _, err := LoadToolArgs(id); err != nil {
				t.Fatal(err)
			}
			paths[strings.TrimSuffix(e.Path, ".md")+".json"] = true
		}
	}
	err := fs.WalkDir(embedded.PromptFiles, "prompts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !paths[path] {
			t.Errorf("unexpected/unregistered embedded file %s", path)
		}
		delete(paths, path)
		return nil
	})
	if err != nil || len(paths) != 0 {
		t.Fatalf("unmatched catalog: %v, %v", paths, err)
	}
}

func TestLoadRejectsInvalidModulesAndKeepsCommandsAndToolsAsText(t *testing.T) {
	if _, err := Load("default"); err == nil {
		t.Fatal("accepted unknown ID")
	}
	if _, err := load(fstest.MapFS{}, "core.agent"); err == nil {
		t.Fatal("accepted missing file")
	}
	if _, err := load(fstest.MapFS{"prompts/core/agent.md": &fstest.MapFile{Data: []byte(" \n ")}}, "core.agent"); err == nil {
		t.Fatal("accepted empty file")
	}
	if _, err := load(fstest.MapFS{"prompts/command/commit.md": &fstest.MapFile{Data: []byte(" \n ")}}, "command.commit"); err == nil {
		t.Fatal("accepted empty command")
	}
	if _, err := load(fstest.MapFS{"prompts/tool/load_component.md": &fstest.MapFile{Data: []byte(" \n ")}}, "tool.load_component"); err == nil {
		t.Fatal("accepted empty tool description")
	}
	const text = "---\nid: unrelated\nunexpected: true\n---\nCommand body."
	for _, id := range []string{"core.agent", "command.commit", "tool.load_component", "jev.intent", "decision.preview"} {
		module, err := load(fstest.MapFS{catalog[id].Path: &fstest.MapFile{Data: []byte(text)}}, id)
		if id != "core.agent" {
			if err != nil || module.ID != id || module.Description != "" || module.Body != text {
				t.Fatalf("prompt %s parsed plain text as metadata: %+v, %v", id, module, err)
			}
		} else if err == nil {
			t.Fatal("accepted invalid module metadata")
		}
	}
}

func TestRenderedBodyRefreshesHash(t *testing.T) {
	source, err := load(fstest.MapFS{"prompts/core/references.md": &fstest.MapFile{Data: []byte("---\nid: core.references\ndescription: '  Reference policy Presentation \"planning\" & <review>  '\n---\n\nReference body.\n")}}, "core.references")
	if err != nil {
		t.Fatal(err)
	}
	if source.Description != `Reference policy Presentation "planning" & <review>` || source.Body != "Reference body." {
		t.Fatalf("description was not parsed separately from the body: %+v", source)
	}
	want := `<system_prompt id="core.references" description="Reference policy Presentation &#34;planning&#34; &amp; &lt;review&gt;">` + "\nReference body.\n</system_prompt>"
	if got := source.SystemPrompt(); got != want {
		t.Fatalf("rendered metadata: got %q, want %q", got, want)
	}
	rendered, err := source.WithBody(source.Body + "\nAn additional rule.")
	if err != nil || rendered.Hash == source.Hash || rendered.Path != source.Path || rendered.Version != source.Version || rendered.Description != source.Description {
		t.Fatalf("render metadata: %+v, %v", rendered, err)
	}
	if _, err := source.WithBody("\n"); err == nil {
		t.Fatal("accepted empty rendered body")
	}
}
