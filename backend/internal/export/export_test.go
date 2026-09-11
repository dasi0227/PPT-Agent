package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func exportFrame() spec.RuntimeFrameContext {
	return spec.RuntimeFrameContext{SlideID: "sli_one", Canvas: spec.CanonicalCanvas(), ThemeID: "theme", DeckTitle: "Deck", Ordinal: 1, Total: 1, Role: "content", Section: spec.RuntimeFrameAncestor{ID: "sec_one", Title: "Section", Index: 1}, Numbering: spec.RuntimeFrameNumbering{Visible: true, Format: "number"}, Chrome: []spec.ChromeItem{{Type: "page_number", Placement: "bottom-right", Style: "muted"}}}
}

func TestRewriteSlideHTMLCreatesStandalonePage(t *testing.T) {
	raw := []byte(`<!doctype html><html><head><link id="base-link" href="/api/v1/runtime/base.css"><link id="theme-link" href="/api/v1/themes/x/css"><script src="/slide-runtime/selection-bridge.js"></script></head><body><div class="slide-stage"><img src="/attachments/att_one/original.png"><div style="background:url('/attachments/att_one/original.png')"></div><a href="https://example.com/read">read</a><script src="https://cdn.example.com/app.js"></script></div></body></html>`)
	attachmentData := map[string]string{"/attachments/att_one/original.png": "data:image/png;base64,eA==", "attachments/att_one/original.png": "data:image/png;base64,eA==", "../attachments/att_one/original.png": "data:image/png;base64,eA=="}
	got, warnings, err := rewriteSlideHTML(raw, exportFrame(), []byte("body{}"), []byte(":root{}"), attachmentData)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{`href="../assets/base.css"`, `href="../assets/theme.css"`, `src="data:image/png;base64,eA=="`, `url(&#39;data:image/png;base64,eA==&#39;)`, `data-runtime-chrome="page_number"`, `src="https://cdn.example.com/app.js"`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	for _, forbidden := range []string{"/api/v1/runtime", "/api/v1/themes", "selection-bridge"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("standalone HTML retained %q", forbidden)
		}
	}
	if len(warnings) != 1 || warnings[0] != "https://cdn.example.com/app.js" {
		t.Fatalf("warnings=%v", warnings)
	}
}

func TestBuildHTMLPackagesOnlyPlaybackFiles(t *testing.T) {
	root := t.TempDir()
	snapshotRoot := filepath.Join(root, "snapshot")
	attachmentRel := "attachments/att_one/original.png"
	if err := writeFile(filepath.Join(snapshotRoot, filepath.FromSlash(attachmentRel)), []byte("image")); err != nil {
		t.Fatal(err)
	}
	op := &Operation{ID: "exp_one", ProjectID: "pro_one", Format: FormatHTML, Status: StatusRunning, TotalPages: 1, subscribers: map[int]chan Event{}, Snapshot: Snapshot{ProjectID: "pro_one", ProjectTitle: "Deck", Root: snapshotRoot, BaseCSS: []byte("body{}"), ThemeCSS: []byte(":root{}"), Slides: []SlideSnapshot{{ID: "sli_one", Title: "One", Ordinal: 1, HTML: []byte(`<html><head></head><body><div class="slide-stage"><img src="/attachments/att_one/original.png"></div></body></html>`), Frame: exportFrame()}}, Attachments: []string{attachmentRel}}}
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
	for _, want := range []string{"index.html", "runtime/player.css", "runtime/player.js", "assets/base.css", "assets/theme.css", "slides/001.html", attachmentRel} {
		if !entries[want] {
			t.Errorf("missing zip entry %s", want)
		}
	}
	for _, forbidden := range []string{"manifest.json", "outline.json", "design.json", "attachments/att_one/meta.json", "attachments/att_one/thumbnail.webp"} {
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

func TestManagerAllowsOnlyOneActiveExportPerProject(t *testing.T) {
	m := NewManager(blockingRenderer{})
	root := filepath.Join(t.TempDir(), "snapshot")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{ProjectID: "pro_one", ProjectTitle: "Deck", Root: root, BaseCSS: []byte("x"), ThemeCSS: []byte("x"), ThemeID: "theme", Slides: []SlideSnapshot{{ID: "sli_one", Title: "One", Ordinal: 1, HTML: []byte("<html></html>"), Frame: exportFrame(), FileName: "001-One.png"}}}
	op, err := m.Start("exp_one", "req_one", FormatPNG, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start("exp_two", "req_two", FormatPNG, snapshot); !errors.Is(err, ErrAlreadyActive) {
		t.Fatalf("err=%v", err)
	}
	if err := m.Cancel(op.ID); err != nil {
		t.Fatal(err)
	}
	if m.Active("pro_one") {
		t.Fatal("project remained active after cancellation")
	}
	if _, err := m.Get(op.ID); !errors.Is(err, ErrGone) {
		t.Fatalf("canceled export remained addressable: %v", err)
	}
}

func TestSafeNameRemovesPathAndControlCharacters(t *testing.T) {
	if got := SafeName(" ../bad:\x00 title? "); strings.ContainsAny(got, "/\\:?\x00") || got == "" {
		t.Fatalf("got %q", got)
	}
}

func TestExternalResourceScanIgnoresOrdinaryLinks(t *testing.T) {
	resources, err := scanResources([]byte(`<html><body><a href="https://example.com">link</a><img src="https://cdn.example.com/a.png"></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0] != "https://cdn.example.com/a.png" {
		t.Fatalf("resources=%v", resources)
	}
}

func TestSnapshotReportsAllMissingSlidesBeforeStarting(t *testing.T) {
	dir, manifest, outline, design := writeSnapshotFixture(t, false)
	_, err := CreateSnapshot(context.Background(), SnapshotInput{ExportID: "exp_one", ProjectID: manifest.ProjectID, ProjectTitle: manifest.Title, ProjectDir: dir, ThemeID: design.Theme, ThemeCSS: []byte(":root{}")})
	var snapshotErr *SnapshotError
	if !errors.As(err, &snapshotErr) || snapshotErr.Code != "EXPORT_SLIDES_MISSING" || len(snapshotErr.Missing) != 2 {
		t.Fatalf("err=%#v", err)
	}
	if snapshotErr.Missing[0].Ordinal != 1 || snapshotErr.Missing[1].Ordinal != 2 {
		t.Fatalf("missing=%v", snapshotErr.Missing)
	}
	_ = outline
}

func TestSnapshotIsFrozenAndDigestChangesWithHTML(t *testing.T) {
	dir, manifest, _, design := writeSnapshotFixture(t, true)
	input := SnapshotInput{ExportID: "exp_one", ProjectID: manifest.ProjectID, ProjectTitle: manifest.Title, ProjectDir: dir, ThemeID: design.Theme, ThemeCSS: []byte(":root{}")}
	first, err := CreateSnapshot(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(first.Root))
	slidePath := filepath.Join(dir, "slides", "sli_aaaaaa", "index.html")
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
	if first.SourceDigest == second.SourceDigest {
		t.Fatal("source digest did not change with slide HTML")
	}
}

func writeSnapshotFixture(t *testing.T, withHTML bool) (string, spec.Manifest, spec.Outline, spec.Design) {
	t.Helper()
	dir := t.TempDir()
	manifest := spec.Manifest{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Title: "Deck", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: spec.CanvasSettings{AspectRatio: spec.CanvasAspectRatio}, Numbering: spec.NumberingPolicy{Enabled: true, HiddenRoles: []string{"cover"}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1}
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: manifest.ProjectID, CreatedAt: 1, UpdatedAt: 1, Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Section", Purpose: "Explain", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "One", Role: spec.SlideRoleContent}, {SlideID: "sli_bbbbbb", Title: "Two", Role: spec.SlideRoleContent}}, Subsections: []spec.Subsection{}}}}
	design := spec.Design{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: manifest.ProjectID, Theme: "theme-one", Direction: "Clear", Density: "medium", Chrome: []spec.ChromeItem{{Type: "page_number", Placement: "bottom-right", Style: "muted"}}, CreatedAt: 1, UpdatedAt: 1}
	for name, value := range map[string]any{"manifest.json": manifest, "outline.json": outline, "design.json": design} {
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if withHTML {
		for _, id := range []string{"sli_aaaaaa", "sli_bbbbbb"} {
			path := filepath.Join(dir, "slides", id, "index.html")
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

func TestStandaloneHTMLWorksFromFileURL(t *testing.T) {
	if os.Getenv("PPT_RUN_CHROMIUM_TEST") != "1" {
		t.Skip("set PPT_RUN_CHROMIUM_TEST=1 to run the standalone HTML integration")
	}
	root := t.TempDir()
	snapshotRoot := filepath.Join(root, "snapshot")
	attachmentRel := "attachments/att_one/original.png"
	pixelImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	pixelImage.Set(0, 0, color.White)
	var pixel bytes.Buffer
	if err := png.Encode(&pixel, pixelImage); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(snapshotRoot, filepath.FromSlash(attachmentRel)), pixel.Bytes()); err != nil {
		t.Fatal(err)
	}
	frame := exportFrame()
	second := frame
	second.SlideID = "sli_two"
	second.Ordinal = 2
	second.Total = 2
	frame.Total = 2
	op := &Operation{ID: "exp_file", ProjectID: "pro_one", Format: FormatHTML, Status: StatusRunning, TotalPages: 2, subscribers: map[int]chan Event{}, Snapshot: Snapshot{ProjectID: "pro_one", ProjectTitle: "Deck", Root: snapshotRoot, BaseCSS: []byte("html,body{margin:0}.slide-stage{width:1920px;height:1080px}"), ThemeCSS: []byte(":root{--proof:green}"), Slides: []SlideSnapshot{{ID: "sli_one", Title: "One", Ordinal: 1, HTML: []byte(`<html><head></head><body><div class="slide-stage"><img id="asset" src="/attachments/att_one/original.png"><script>document.body.dataset.script='ok'</script></div></body></html>`), Frame: frame}, {ID: "sli_two", Title: "Two", Ordinal: 2, HTML: []byte(`<html><head></head><body><div class="slide-stage" id="second">two</div></body></html>`), Frame: second}}, Attachments: []string{attachmentRel}}}
	artifact, _, failure := buildHTML(context.Background(), op)
	if failure != nil {
		t.Fatalf("failure=%v", failure)
	}
	extracted := filepath.Join(root, "extracted")
	if err := os.MkdirAll(extracted, 0o700); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(artifact.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range reader.File {
		target := filepath.Join(extracted, filepath.FromSlash(entry.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		source, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(target)
		if err != nil {
			t.Fatal(err)
		}
		_, copyErr := io.Copy(out, source)
		_ = source.Close()
		_ = out.Close()
		if copyErr != nil {
			t.Fatal(copyErr)
		}
	}
	_ = reader.Close()
	script := `import {chromium} from 'playwright-core';import {pathToFileURL} from 'node:url';const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});const page=await browser.newPage();await page.goto(pathToFileURL(process.argv[1]).href);await page.waitForTimeout(500);let frame=page.frames()[1];const first=await frame.evaluate(()=>({script:document.body.dataset.script,img:document.querySelector('#asset')?.naturalWidth,styles:document.styleSheets.length}));if(first.script!=='ok'||first.img!==1||first.styles<2)throw new Error(JSON.stringify(first));await page.keyboard.press('ArrowRight');await page.waitForTimeout(300);frame=page.frames()[1];if(!await frame.$('#second'))throw new Error('navigation failed');await browser.close();`
	command := exec.Command("node", "--input-type=module", "-e", script, filepath.Join(extracted, "index.html"))
	command.Dir = filepath.Join("..", "..", "render-worker")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("file player failed: %v: %s", err, output)
	}
}
