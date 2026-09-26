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
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func exportFrame() spec.RuntimeFrameContext {
	return spec.RuntimeFrameContext{Appearance: runtimeassets.Appearance("theme", []byte(`:root{--font-mono:"JetBrains Mono";--font-sans:"Noto Sans SC";--color-caption:#666;--color-fg:#222;}`)), SlideID: "sli_one", Canvas: spec.CanonicalCanvas(), ThemeID: "theme", DeckTitle: "Deck", Ordinal: 1, Total: 1, Role: "content", Section: spec.RuntimeFrameAncestor{ID: "sec_one", Title: "Section", Index: 1}, Decorations: spec.Decorations{PageNumber: "bottom-right", DeckTitle: "none", SectionTitle: "none", KeyMessage: "none"}}
}

func TestRewriteSlideHTMLCreatesStandalonePage(t *testing.T) {
	raw := []byte(`<!doctype html><html><head><link id="base-link" href="/api/v1/runtime/base.css"><link id="theme-link" href="/api/v1/themes/x/css"><script src="/slide-runtime/selection-bridge.js"></script></head><body><div class="slide-stage"><img src="/attachments/att_one/original.png"><div style="background:url('/attachments/att_one/original.png')"></div><a href="https://example.com/read">read</a><script src="https://cdn.example.com/app.js"></script></div></body></html>`)
	attachmentData := map[string]string{"/attachments/att_one/original.png": "data:image/png;base64,eA==", "attachments/att_one/original.png": "data:image/png;base64,eA==", "../attachments/att_one/original.png": "data:image/png;base64,eA=="}
	got, warnings, err := rewriteSlideHTML(raw, []byte("body{}"), []byte(":root{}"), attachmentData)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{`href="../assets/fonts.css"`, `src="data:image/png;base64,eA=="`, `url(&#39;data:image/png;base64,eA==&#39;)`, `<style id="export-base-inline">body{}</style>`, `<style id="export-theme-inline">:root{}</style>`, `src="https://cdn.example.com/app.js"`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	for _, forbidden := range []string{"/api/v1/runtime", "/api/v1/themes", "selection-bridge", "data-runtime-decoration"} {
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
	if err := runtimeassets.Materialize(filepath.Join(snapshotRoot, "runtime-assets")); err != nil {
		t.Fatal(err)
	}
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
	for _, want := range []string{"index.html", "runtime/player.css", "runtime/player.js", "runtime/decorations.js", "assets/base.css", "assets/theme.css", "assets/fonts.css", "assets/fonts/notosanssc-OFL.txt", "slides/001.html", attachmentRel} {
		if !entries[want] {
			t.Errorf("missing zip entry %s", want)
		}
	}
	for _, forbidden := range []string{".manifest.json", ".outline.json", ".design.json", "attachments/att_one/meta.json", "attachments/att_one/thumbnail.webp"} {
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

func TestExportViewSerializesEmptyWarningsAsArray(t *testing.T) {
	op := &Operation{}
	view := op.View()
	if view.Warnings == nil || !strings.Contains(eventPayload(view), `"warnings":[]`) {
		t.Fatalf("empty warning contract broken: %s", eventPayload(view))
	}
}

func TestCompletedAndCanceledExportsRemoveEntireTemporaryDirectory(t *testing.T) {
	for _, download := range []bool{false, true} {
		name := "cancel"
		if download {
			name = "download"
		}
		t.Run(name, func(t *testing.T) {
			m := NewManager(blockingRenderer{})
			defer m.Close()
			root := filepath.Join(t.TempDir(), "export", "snapshot")
			if err := writeFile(filepath.Join(root, "marker"), []byte("snapshot")); err != nil {
				t.Fatal(err)
			}
			op, err := m.Start("exp_cleanup", "req_cleanup", FormatHTML, Snapshot{ProjectID: "pro_one", ProjectTitle: "Deck", Root: root})
			if err != nil {
				t.Fatal(err)
			}
			<-op.done
			if op.View().Status != StatusReady {
				t.Fatalf("view=%+v", op.View())
			}
			if download {
				if _, err := m.BeginDelivery(op.ID); err != nil {
					t.Fatal(err)
				}
				m.Consume(op)
			} else if err := m.Cancel(op.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Dir(root)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("snapshot, intermediate files or artifact remain: %v", err)
			}
			if m.Active("pro_one") {
				t.Fatal("project still reserved")
			}
			if _, err := m.Get(op.ID); !errors.Is(err, ErrGone) {
				t.Fatalf("export still retained: %v", err)
			}
		})
	}
}

func TestAbandonedExportReleasesProjectAndRemovesFiles(t *testing.T) {
	for _, format := range []Format{FormatHTML, FormatPNG} {
		t.Run(string(format), func(t *testing.T) {
			m := NewManager(blockingRenderer{})
			defer m.Close()
			root := filepath.Join(t.TempDir(), "export", "snapshot")
			snapshot := Snapshot{ProjectID: "pro_one", ProjectTitle: "Deck", Root: root, Slides: []SlideSnapshot{{ID: "sli_one", Ordinal: 1, HTML: []byte("<html><body></body></html>"), Frame: exportFrame(), FileName: "001-One.png"}}}
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
			fresh, err := m.Start("exp_new", "req_new", FormatHTML, Snapshot{ProjectID: "pro_one", Root: filepath.Join(t.TempDir(), "new", "snapshot")})
			if err != nil {
				t.Fatal(err)
			}
			<-fresh.done
		})
	}
}

func TestPageExpiryDoesNotInterruptDownloadButTTLStillCleansIt(t *testing.T) {
	m := NewManager(nil)
	defer m.Close()
	op, err := m.Start("exp_one", "req_one", FormatHTML, Snapshot{ProjectID: "pro_one", Root: filepath.Join(t.TempDir(), "export", "snapshot")})
	if err != nil {
		t.Fatal(err)
	}
	<-op.done
	if _, err := m.BeginDelivery(op.ID); err != nil {
		t.Fatal(err)
	}
	if m.expireAt(op.ID, time.Now().Add(heartbeatGrace+time.Second)) {
		t.Fatal("page heartbeat interrupted download")
	}
	if !m.expireAt(op.ID, time.Now().Add(defaultTTL)) {
		t.Fatal("TTL did not clean stalled download")
	}
	if m.Active("pro_one") {
		t.Fatal("expired download kept project occupied")
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
	dir, manifest, outline, _ := writeSnapshotFixture(t, false)
	_, err := CreateSnapshot(context.Background(), SnapshotInput{ExportID: "exp_one", ProjectID: "pro_aaaaaa", ProjectTitle: manifest.Title, ProjectDir: dir, ThemeID: "theme-one", ThemeCSS: []byte(":root{}")})
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
	dir, manifest, _, _ := writeSnapshotFixture(t, true)
	input := SnapshotInput{ExportID: "exp_one", ProjectID: "pro_aaaaaa", ProjectTitle: manifest.Title, ProjectDir: dir, ThemeID: "theme-one", ThemeCSS: []byte(":root{}")}
	first, err := CreateSnapshot(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(first.Root))
	for _, name := range []string{".manifest.json", ".design.json", ".outline.json", ".spec.json"} {
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
	if first.SourceDigest == second.SourceDigest {
		t.Fatal("source digest did not change with slide HTML")
	}
}

func writeSnapshotFixture(t *testing.T, withHTML bool) (string, spec.Manifest, spec.Outline, spec.Design) {
	t.Helper()
	dir := t.TempDir()
	manifest := spec.Manifest{Title: "Deck", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}}
	outline := spec.Outline{Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Section", Purpose: "Explain", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "One"}, {SlideID: "sli_bbbbbb", Title: "Two"}}, Subsections: []spec.Subsection{}}}}
	design := spec.Design{Direction: "Clear", LayoutPreferences: []string{}, Decorations: spec.Decorations{PageNumber: "bottom-right", DeckTitle: "none", SectionTitle: "none", KeyMessage: "none"}}
	for name, value := range map[string]any{".manifest.json": manifest, ".outline.json": outline, ".design.json": design, ".spec.json": map[string]spec.SlideSpec{}} {
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

func TestStandaloneHTMLWorksFromFileURL(t *testing.T) {
	if os.Getenv("PPT_RUN_CHROMIUM_TEST") != "1" {
		t.Skip("set PPT_RUN_CHROMIUM_TEST=1 to run the standalone HTML integration")
	}
	root := t.TempDir()
	snapshotRoot := filepath.Join(root, "snapshot")
	if err := runtimeassets.Materialize(filepath.Join(snapshotRoot, "runtime-assets")); err != nil {
		t.Fatal(err)
	}
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
	frame.Decorations.SectionTitle = "top-left"
	frame.Decorations.DeckTitle = "bottom-left"
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
	// Deliberately remove iframe-linked styles: the inline copy and outer decorations
	// must still render correctly when local sandbox resource loading is unavailable.
	for _, name := range []string{"base.css", "theme.css"} {
		if err := os.Remove(filepath.Join(extracted, "assets", name)); err != nil {
			t.Fatal(err)
		}
	}
	script := `
import {chromium} from 'playwright-core';
import {pathToFileURL} from 'node:url';
const browser = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
  const page = await browser.newPage();
  await page.goto(pathToFileURL(process.argv[1]).href);
  let frame = page.frames()[1];
  await frame.waitForSelector('#asset');
  const first = await frame.evaluate(() => ({
    script: document.body.dataset.script, img: document.querySelector('#asset').naturalWidth,
    width: getComputedStyle(document.querySelector('.slide-stage')).width,
    decorations: document.querySelectorAll('[data-runtime-decoration]').length,
  }));
  if (first.script !== 'ok' || first.img !== 1 || first.width !== '1920px' || first.decorations !== 0) throw new Error(JSON.stringify(first));
  const outer = await page.evaluate(() => {
    const marker = document.querySelector('[data-runtime-decoration="section_title"]');
    return {position: getComputedStyle(marker).position, fontSize: getComputedStyle(marker).fontSize,
      parent: marker.parentElement.id, controls: document.querySelectorAll('nav,button').length,
      number: document.querySelector('[data-runtime-page-number]')?.textContent,
      stageHeight: document.getElementById('stage').clientHeight, viewportHeight: innerHeight};
  });
  if (outer.position !== 'absolute' || outer.fontSize !== '16px' || outer.parent !== 'canvas' || outer.controls || outer.number !== '1' || outer.stageHeight !== outer.viewportHeight) throw new Error(JSON.stringify(outer));
  // Keyboard navigation must keep working after the user clicks into the iframe.
  await frame.locator('.slide-stage').click({position:{x:200,y:200}});
  await page.keyboard.press('ArrowRight');
  await page.waitForFunction(() => document.querySelector('[data-runtime-page-number]')?.textContent === '2');
  frame = page.frames()[1];
  await frame.waitForSelector('#second');
  await frame.locator('#second').click({position:{x:200,y:200}});
  await page.keyboard.press('Home');
  await page.waitForFunction(() => document.getElementById('slide').getAttribute('src') === 'slides/001.html');
} finally { await browser.close(); }
`
	command := exec.Command("node", "--input-type=module", "-e", script, filepath.Join(extracted, "index.html"))
	command.Dir = filepath.Join("..", "..", "render-worker")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("file player failed: %v: %s", err, output)
	}
}

func TestOfflineFontsEmbedSnapshotBytesAndKeepLicenses(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	if err := writeFile(filepath.Join(source, "fonts.css"), []byte(`@font-face{src:url("fonts/Test.ttf")}`)); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(source, "fonts", "Test.ttf"), []byte("frozen-font")); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(source, "fonts", "OFL.txt"), []byte("license")); err != nil {
		t.Fatal(err)
	}
	if err := writeOfflineFonts(source, target); err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile(filepath.Join(target, "fonts.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "data:font/ttf;base64,ZnJvemVuLWZvbnQ=") || strings.Contains(string(css), "fonts/Test.ttf") {
		t.Fatal("offline CSS does not contain the snapshot font")
	}
	license, err := os.ReadFile(filepath.Join(target, "fonts", "OFL.txt"))
	if err != nil || string(license) != "license" {
		t.Fatal("font license missing")
	}
}
