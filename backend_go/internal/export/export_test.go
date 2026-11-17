package export

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func exportFrame() spec.RuntimeFrameContext {
	return spec.RuntimeFrameContext{Appearance: runtimeassets.Appearance("theme", []byte(`:root{--font-mono:"JetBrains Mono";--font-sans:"Noto Sans SC";--color-caption:#666;--color-fg:#222;}`)), SlideID: "sli_one", Canvas: spec.CanonicalCanvas(), ThemeID: "theme", DeckTitle: "Deck", Ordinal: 1, Total: 1, Purpose: "content_explanation", Section: spec.RuntimeFrameAncestor{ID: "sec_one", Title: "Section", Index: 1}, Decorations: decoration.Positions{SlidePagination: "bottom-right", DeckTitle: "none", SectionTitle: "none", Logo: "none"}}
}

func TestBuildHTMLPackagesOnlyPlaybackFiles(t *testing.T) {
	root := t.TempDir()
	snapshotRoot := filepath.Join(root, "snapshot")
	if err := runtimeassets.Materialize(filepath.Join(snapshotRoot, "runtime-assets")); err != nil {
		t.Fatal(err)
	}
	attachmentRel := "attachments/att_one.png"
	if err := writeFile(filepath.Join(snapshotRoot, filepath.FromSlash(attachmentRel)), []byte("image")); err != nil {
		t.Fatal(err)
	}
	op := &Operation{ID: "exp_one", ProjectID: "pro_one", Format: FormatHTML, Status: StatusRunning, TotalPages: 1, subscribers: map[int]chan Event{}, Snapshot: Snapshot{ProjectID: "pro_one", ProjectTitle: "Deck", Root: snapshotRoot, BaseCSS: []byte("body{}"), ThemeCSS: []byte(":root{}"), Slides: []SlideSnapshot{{ID: "sli_one", Title: "One", Ordinal: 1, HTML: []byte(`<html><head></head><body><div class="slide-stage"><img src="/attachments/att_one.png"></div></body></html>`), Frame: exportFrame()}}, Attachments: []string{attachmentRel}}}
	artifact, _, failure := buildHTML(context.Background(), op)
	if failure != nil {
		t.Fatalf("failure=%v", failure)
	}
	reader, err := zip.OpenReader(artifact.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	entries := map[string]bool{}
	for _, entry := range reader.File {
		entries[entry.Name] = true
	}
	for _, want := range []string{"index.html", "runtime/player.css", "runtime/player.js", "runtime/decorations.js", "assets/base.css", "assets/theme.css", "assets/fonts.css", "assets/fonts/notosanssc-OFL.txt", "slides/001.html", attachmentRel} {
		if !entries[want] {
			t.Errorf("missing zip entry %s", want)
		}
	}
	for _, forbidden := range []string{".meta.json", ".outline.json", ".constraints.json", "attachments/att_one/meta.json", "attachments/att_one.webp"} {
		if entries[forbidden] {
			t.Errorf("unexpected zip entry %s", forbidden)
		}
	}
}

type blockingRenderer struct{}

func (blockingRenderer) Render(ctx context.Context, _ workflow.RenderRequest) (workflow.RenderDiagnostics, error) {
	<-ctx.Done()
	return workflow.RenderDiagnostics{}, context.Cause(ctx)
}

func (blockingRenderer) AssemblePDF(context.Context, []string, string) (int, error) {
	return 0, errors.New("not reached")
}

func exportLifecycleSnapshot(t *testing.T, root string) Snapshot {
	t.Helper()
	if err := runtimeassets.Materialize(filepath.Join(root, "runtime-assets")); err != nil {
		t.Fatal(err)
	}
	return Snapshot{ProjectID: "pro_one", ProjectTitle: "Deck", Root: root,
		BaseCSS: []byte("body{}"), ThemeCSS: []byte(":root{}"), ThemeID: "theme",
		Slides: []SlideSnapshot{{ID: "sli_one", Title: "One", Ordinal: 1,
			HTML:  []byte(`<html><body><div class="slide-stage">One</div></body></html>`),
			Frame: exportFrame(), FileName: "001-One.png"}},
	}
}

func TestAbandonedExportReleasesProjectAndRemovesFiles(t *testing.T) {
	for _, format := range []Format{FormatHTML, FormatPNG} {
		t.Run(string(format), func(t *testing.T) {
			m := NewManager(blockingRenderer{})
			defer m.Close()
			root := filepath.Join(t.TempDir(), "export", "snapshot")
			snapshot := exportLifecycleSnapshot(t, root)
			op, err := m.Start("exp_one", "req_one", format, snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if format == FormatHTML {
				<-op.done
				if op.View().Status != StatusReady {
					t.Fatalf("view=%+v", op.View())
				}
			}
			if err := m.Heartbeat(op.ID); err != nil {
				t.Fatal(err)
			}
			if m.expireAt(op.ID, time.Now().Add(heartbeatGrace/2)) || !m.Active("pro_one") {
				t.Fatal("live export must keep its project reservation")
			}
			if !m.expireAt(op.ID, time.Now().Add(heartbeatGrace+time.Second)) {
				t.Fatal("abandoned export did not expire")
			}
			if m.Active("pro_one") {
				t.Fatal("abandoned export blocked a fresh export")
			}
			if err := m.Heartbeat(op.ID); !errors.Is(err, ErrGone) {
				t.Fatalf("late heartbeat=%v", err)
			}
			if _, err := os.Stat(filepath.Dir(root)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("orphan files remain: %v", err)
			}
			fresh, err := m.Start("exp_new", "req_new", FormatHTML, exportLifecycleSnapshot(t, filepath.Join(t.TempDir(), "new", "snapshot")))
			if err != nil {
				t.Fatal(err)
			}
			<-fresh.done
		})
	}
}

func TestSnapshotIsFrozenAcrossHTMLChanges(t *testing.T) {
	dir, manifest, _, _ := writeSnapshotFixture(t, true)
	if err := os.WriteFile(filepath.Join(dir, ".sli_aaaaaa.json"), []byte("{\n  \"purpose\": \"content_explanation\", \"layout\": \"Title above content\", \"elements\": [], \"core\": \"Original\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := SnapshotInput{Decorations: decoration.DefaultPositions(), ExportID: "exp_one", ProjectID: "pro_aaaaaa", ProjectTitle: manifest.Title, ProjectDir: dir, ThemeID: "theme-one", ThemeCSS: []byte(":root{}")}
	first, err := CreateSnapshot(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(first.Root))
	for _, name := range []string{".meta.json", ".constraints.json", ".outline.json", ".sli_aaaaaa.json"} {
		original, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		frozen, err := os.ReadFile(filepath.Join(first.Root, name))
		if err != nil || string(original) != string(frozen) {
			t.Fatalf("hidden authoring file omitted from export snapshot: %s %v", name, err)
		}
	}
	slidePath := filepath.Join(dir, "sli_aaaaaa"+".html")
	if err := os.WriteFile(slidePath, []byte(`<html><head></head><body><div class="slide-stage">changed</div></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(first.Slides[0].HTML), "changed") {
		t.Fatal("snapshot changed with authoring workspace")
	}
	input.ExportID = "exp_two"
	second, err := CreateSnapshot(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(second.Root))
	if !strings.Contains(string(second.Slides[0].HTML), "changed") {
		t.Fatal("new snapshot did not capture the current slide HTML")
	}
}

func writeSnapshotFixture(t *testing.T, withHTML bool) (string, spec.Meta, spec.Outline, spec.Constraints) {
	t.Helper()
	dir := t.TempDir()
	manifest := spec.Meta{Title: "Deck", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Count: "待明确"}
	outline := spec.Outline{Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Section", Slides: []spec.SlideNode{{ID: "sli_aaaaaa", Title: "One"}, {ID: "sli_bbbbbb", Title: "Two"}}, Subsections: []spec.Subsection{}}}}
	design := spec.Constraints{Content: spec.ConstraintSet{Requirements: []string{}, Prohibitions: []string{}}, Visual: spec.ConstraintSet{Requirements: []string{"Clear"}, Prohibitions: []string{}}}
	for name, value := range map[string]any{".meta.json": manifest, ".outline.json": outline, ".constraints.json": design} {
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if withHTML {
		for _, id := range []string{"sli_aaaaaa", "sli_bbbbbb"} {
			path := filepath.Join(dir, id+".html")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(`<html><head></head><body><div class="slide-stage">original</div></body></html>`), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir, manifest, outline, design
}
