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
		if m.Version == "" || m.ID != id || m.Desc == "" || m.Path != e.Path || m.Hash != fmt.Sprintf("%x", sha256.Sum256([]byte(m.Render()))) {
			t.Fatalf("invalid module metadata: %+v", m)
		}
		if strings.HasPrefix(m.Body, "---") {
			t.Fatalf("prompt %s leaked front matter into its body", id)
		}
	}
	tools, err := fs.ReadDir(embedded.PromptFiles, "prompts/tool")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range tools {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(file.Name(), ".md")
		description, _, err := LoadTool(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(description, "---") || strings.HasPrefix(description, "<tool_") {
			t.Fatalf("tool %s contains module metadata", name)
		}
		paths["prompts/tool/"+name+".md"] = true
		paths["prompts/tool/"+name+".json"] = true
	}
	err = fs.WalkDir(embedded.PromptFiles, "prompts", func(path string, d fs.DirEntry, err error) error {
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

func TestLoadValidatesModuleAndToolSources(t *testing.T) {
	if _, err := Load("default"); err == nil {
		t.Fatal("accepted unknown ID")
	}
	if _, err := load(fstest.MapFS{}, "core_agent"); err == nil {
		t.Fatal("accepted missing file")
	}
	if _, err := load(fstest.MapFS{"prompts/core/agent.md": &fstest.MapFile{Data: []byte(" \n ")}}, "core_agent"); err == nil {
		t.Fatal("accepted empty file")
	}
	if _, err := load(fstest.MapFS{"prompts/command/commit.md": &fstest.MapFile{Data: []byte(" \n ")}}, "command_commit"); err == nil {
		t.Fatal("accepted empty command")
	}
	toolFiles := fstest.MapFS{
		"prompts/tool/load_component.md":   &fstest.MapFile{Data: []byte("\nLoad the component.\n\nKeep its HTML unchanged.\n")},
		"prompts/tool/load_component.json": &fstest.MapFile{Data: []byte(`{"component_id":"Component ID."}`)},
	}
	description, arguments, err := loadTool(toolFiles, "load_component")
	if err != nil || description != "Load the component.\n\nKeep its HTML unchanged." || arguments["component_id"] != "Component ID." {
		t.Fatalf("invalid plain tool description: %q, %v, %v", description, arguments, err)
	}
	toolFiles["prompts/tool/load_component.md"].Data = []byte(" \n ")
	if _, _, err := loadTool(toolFiles, "load_component"); err == nil {
		t.Fatal("accepted empty tool description")
	}
	for _, id := range []string{"core_agent", "reminder_constraint_boundaries", "command_commit", "jev_intent", "decision_preview"} {
		for _, text := range []string{
			"Body without metadata.",
			"---\nid: " + id,
			"---\nid: unrelated\ndesc: Wrong ID.\n---\nBody.",
			"---\nid: " + id + "\ndesc: Valid description.\nunexpected: true\n---\nBody.",
			"---\nid: " + id + "\ndescription: Old metadata.\n---\nBody.",
			"---\nid: " + id + "\ndesc: ''\n---\nBody.",
			"---\nid: " + id + "\ndesc: Empty body.\n---\n",
		} {
			if _, err := load(fstest.MapFS{catalog[id].Path: &fstest.MapFile{Data: []byte(text)}}, id); err == nil {
				t.Fatalf("prompt %s accepted invalid module %q", id, text)
			}
		}
	}
	if _, err := Load("core.agent"); err == nil {
		t.Fatal("accepted a legacy dotted ID")
	}
}

func TestRenderedBodyRefreshesHash(t *testing.T) {
	source, err := load(fstest.MapFS{"prompts/core/references.md": &fstest.MapFile{Data: []byte("---\nid: core_references\ndesc: '  Reference policy Presentation \"planning\" & <review>  '\n---\n\nReference body.\n")}}, "core_references")
	if err != nil {
		t.Fatal(err)
	}
	if source.Desc != `Reference policy Presentation "planning" & <review>` || source.Body != "Reference body." {
		t.Fatalf("description was not parsed separately from the body: %+v", source)
	}
	want := `<core_references desc="Reference policy Presentation &#34;planning&#34; &amp; &lt;review&gt;">` + "\nReference body.\n</core_references>"
	if got := source.Render(); got != want {
		t.Fatalf("rendered metadata: got %q, want %q", got, want)
	}
	rendered, err := source.WithBody(source.Body + "\nAn additional rule.")
	if err != nil || rendered.Hash == source.Hash || rendered.Path != source.Path || rendered.Version != source.Version || rendered.Desc != source.Desc {
		t.Fatalf("render metadata: %+v, %v", rendered, err)
	}
	if _, err := source.WithBody("\n"); err == nil {
		t.Fatal("accepted empty rendered body")
	}
	metadataChanged := source
	metadataChanged.Desc = "Updated description."
	metadataChanged, err = metadataChanged.WithBody(source.Body)
	if err != nil || metadataChanged.Hash == source.Hash {
		t.Fatal("model-facing metadata changes must refresh the hash")
	}
	if got := RenderGroup("system_prompt", source); got != "<system_prompt>\n"+want+"\n</system_prompt>" {
		t.Fatalf("incorrect group structure: %s", got)
	}
}
