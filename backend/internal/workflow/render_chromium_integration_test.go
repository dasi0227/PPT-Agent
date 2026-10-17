package workflow

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const (
	validToolHTML = `<!doctype html><html lang="zh"><head><link id="base-link" rel="stylesheet" href="/api/v1/runtime/base.css"><link id="theme-link" rel="stylesheet" href="/api/v1/themes/swiss-modern/css"></head><body><section class="slide-stage"><h1>Original</h1></section></body></html>`
	testBaseCSS   = `html,body{margin:0}.slide-stage{width:1920px;height:1080px;overflow:hidden}`
	testThemeCSS  = `:root{--theme-proof:37px}`
)

func testRenderFrame() spec.RuntimeFrameContext {
	return spec.RuntimeFrameContext{Appearance: runtimeassets.Appearance("swiss-modern", []byte(testThemeCSS)), SlideID: "slide-01", Canvas: spec.CanonicalCanvas(), ThemeID: "swiss-modern", DeckTitle: "Deck", Ordinal: 2, Total: 2, Purpose: "content", Section: spec.RuntimeFrameAncestor{ID: "sec_test", Title: "Section", Index: 1}, Decorations: decoration.Positions{SlidePagination: "bottom-right", DeckTitle: "top-right", SectionTitle: "top-left", Logo: "none"}}
}

func TestNodeSlideRendererWithRealChromium(t *testing.T) {
	if os.Getenv("PPT_RUN_CHROMIUM_TEST") != "1" {
		t.Skip("set PPT_RUN_CHROMIUM_TEST=1 to run the real Chromium integration")
	}
	renderer := NewNodeSlideRenderer(NodeRendererConfig{
		WorkerPath: filepath.Join("..", "..", "render-worker", "worker.mjs"),
		Timeout:    30 * time.Second,
	})
	defer renderer.Close()
	if err := renderer.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	renderer.mu.Lock()
	initialPID := renderer.cmd.Process.Pid
	renderer.mu.Unlock()
	dir := t.TempDir()
	for _, rel := range []string{"."} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_, invalidViewportErr := renderer.Render(context.Background(), RenderRequest{RuntimeAssetsDir: testRuntimeAssets(t),
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: validToolHTML,
		ScreenshotPath: filepath.Join(dir, "invalid-viewport.png"),
		ViewportWidth:  1600, ViewportHeight: 900, TimeoutMS: 15000, Frame: testRenderFrame(),
		BaseCSS: testBaseCSS, ThemeID: "swiss-modern", ThemeCSS: testThemeCSS,
	})
	if invalidViewportErr == nil || !strings.Contains(invalidViewportErr.Error(), "render viewport must match the runtime canvas") {
		t.Fatalf("non-canonical viewport accepted: %v", invalidViewportErr)
	}
	screenshot := filepath.Join(dir, "shot.png")
	diagnostics, err := renderer.Render(context.Background(), RenderRequest{RuntimeAssetsDir: testRuntimeAssets(t),
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01",
		HTML:           strings.Replace(validToolHTML, "</body>", `<script>window.pageState='yes'</script></body>`, 1),
		ScreenshotPath: screenshot, ViewportWidth: 1920, ViewportHeight: 1080, TimeoutMS: 15000, Frame: testRenderFrame(),
		BaseCSS: testBaseCSS, ThemeID: "swiss-modern", ThemeCSS: testThemeCSS,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.ScreenshotBytes <= 0 || diagnostics.FontStatus != "loaded" {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	if diagnostics.ContentSize["width"] != 1920 || diagnostics.ContentSize["height"] != 1080 ||
		diagnostics.Overflow["horizontal"] || diagnostics.Overflow["vertical"] {
		t.Fatalf("canonical stage diagnostics=%+v", diagnostics)
	}
	if strings.Join(diagnostics.RuntimeDecorations, ",") != "slide_pagination,deck_title,section_title" {
		t.Fatalf("runtime decorations was not injected: %#v", diagnostics.RuntimeDecorations)
	}
	if _, err := os.Stat(screenshot); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(screenshot)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(file)
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 1920 || config.Height != 1080 {
		t.Fatalf("screenshot size=%dx%d", config.Width, config.Height)
	}
	// This opt-in browser regression is run manually, never by ordinary unit tests.
	for _, tc := range []struct {
		name, html, direction string
	}{
		{"vertical", `<div id="clip" style="width:300px;height:30px;overflow:hidden;font:20px/20px Arial"><span>First line</span><br><span id="cut">Required last line</span></div>`, "bottom"},
		{"horizontal", `<div id="clip" style="width:40px;overflow:hidden;white-space:nowrap;font:20px/30px Arial"><span id="cut">Required long text</span></div>`, "right"},
		{"scroll", `<div id="clip" style="width:300px;height:30px;overflow:auto;font:20px/20px Arial"><div><span>First line</span><br><span id="cut">Required last line</span></div></div>`, "bottom"},
		{"fixed-layout", `<div id="clip" style="width:300px;height:70px;overflow:hidden;font:20px/20px Arial"><span>First line</span><br><span id="cut">Required last line</span></div>`, ""},
		{"decoration", `<div id="clip" style="width:40px;height:40px;overflow:hidden"><svg width="200" height="200"><rect width="200" height="200" fill="red"/></svg></div>`, ""},
		{"hidden-text", `<div id="clip" style="width:40px;height:10px;overflow:hidden"><span style="visibility:hidden">Hidden text</span><span style="opacity:0">Invisible text</span></div>`, ""},
		{"positioned-escape", `<div style="position:relative"><div id="clip" style="width:40px;height:40px;overflow:hidden"><span style="position:absolute;top:80px;left:80px">Escapes this overflow box</span></div></div>`, ""},
	} {
		t.Run("text-clipping-"+tc.name, func(t *testing.T) {
			d, err := renderer.Render(context.Background(), RenderRequest{RuntimeAssetsDir: testRuntimeAssets(t),
				RunID: "integration", ProjectDir: dir, SlideID: "slide-01",
				HTML:           strings.Replace(validToolHTML, "<h1>Original</h1>", tc.html, 1),
				ScreenshotPath: filepath.Join(dir, tc.name+".png"), ViewportWidth: 1920, ViewportHeight: 1080,
				TimeoutMS: 15000, Frame: testRenderFrame(), BaseCSS: testBaseCSS, ThemeID: "swiss-modern", ThemeCSS: testThemeCSS,
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.direction == "" {
				if len(d.TextClipping) != 0 {
					t.Fatalf("unclipped content reported: %+v", d.TextClipping)
				}
				return
			}
			if len(d.TextClipping) != 1 {
				t.Fatalf("expected one clipped fragment: %+v", d)
			}
			clip := d.TextClipping[0]
			container, _ := clip["container"].(map[string]any)
			directions, _ := clip["directions"].([]any)
			if container["id"] != "clip" || len(directions) != 1 || directions[0] != tc.direction || clip["text"] == "" || clip["clipped_px"] == nil {
				t.Fatalf("clipping location or measurement missing: %+v", clip)
			}
			result := slideRenderResult(renderimage.Entry{SlideID: "slide-01"}, RenderProof{}, d, "test", false)
			if result.OK || len(result.Issues) != 1 || result.Issues[0].Code != "RENDER_TEXT_CLIPPING" {
				t.Fatalf("container clipping inside canvas did not block completion: %+v", result)
			}
		})
	}
	pdfPath := filepath.Join(dir, "deck.pdf")
	pageCount, err := renderer.AssemblePDF(context.Background(), []string{screenshot, screenshot}, pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if pageCount != 2 {
		t.Fatalf("PDF page count=%d", pageCount)
	}
	if info, statErr := os.Stat(pdfPath); statErr != nil || info.Size() == 0 {
		t.Fatalf("PDF was not created: info=%v err=%v", info, statErr)
	}

	secondScreenshot := filepath.Join(dir, "shot-2.png")
	secondHTML := `<!doctype html><html><body><section class="slide-stage"><h1>Second</h1><script>if(window.pageState)console.error('context leaked');try{void parent.document.body;console.error('iframe not isolated')}catch{}</script></section></body></html>`
	diagnostics, err = renderer.Render(context.Background(), RenderRequest{RuntimeAssetsDir: testRuntimeAssets(t),
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: secondHTML,
		ScreenshotPath: secondScreenshot, ViewportWidth: 1920, ViewportHeight: 1080, TimeoutMS: 15000, Frame: testRenderFrame(),
		BaseCSS: testBaseCSS, ThemeID: "swiss-modern", ThemeCSS: testThemeCSS,
	})
	if err != nil || len(diagnostics.ConsoleErrors) != 0 {
		t.Fatalf("isolated second render failed: diagnostics=%+v err=%v", diagnostics, err)
	}
	renderer.mu.Lock()
	reusedPID := renderer.cmd.Process.Pid
	renderer.mu.Unlock()
	if reusedPID != initialPID {
		t.Fatalf("browser worker was not reused: first=%d second=%d", initialPID, reusedPID)
	}
	runtimeCSSHTML := `<!doctype html><html><head><link id="base-link" rel="stylesheet" href="/api/v1/runtime/base.css"><link id="theme-link" rel="stylesheet" href="/api/v1/themes/swiss-modern/css"></head><body><section class="slide-stage"><div id="proof" style="width:var(--theme-proof)">Runtime CSS</div></section><script>if(getComputedStyle(document.querySelector('#proof')).width!=='37px')console.error('runtime css missing')</script></body></html>`
	runtimeCSSDiagnostics, runtimeCSSErr := renderer.Render(context.Background(), RenderRequest{RuntimeAssetsDir: testRuntimeAssets(t),
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: runtimeCSSHTML,
		ScreenshotPath: filepath.Join(dir, "overlay.png"), ViewportWidth: 1920, ViewportHeight: 1080, TimeoutMS: 15000, Frame: testRenderFrame(),
		BaseCSS: testBaseCSS, ThemeID: "swiss-modern", ThemeCSS: testThemeCSS,
	})
	if runtimeCSSErr != nil || len(runtimeCSSDiagnostics.ConsoleErrors) != 0 {
		t.Fatalf("runtime CSS was not served: diagnostics=%+v err=%v", runtimeCSSDiagnostics, runtimeCSSErr)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, timeoutErr := renderer.Render(timeoutCtx, RenderRequest{RuntimeAssetsDir: testRuntimeAssets(t),
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01",
		HTML:           `<html><body><section class="slide-stage"><script>while(true){}</script></section></body></html>`,
		ScreenshotPath: filepath.Join(dir, "timeout.png"), ViewportWidth: 1920, ViewportHeight: 1080, TimeoutMS: 1000, Frame: testRenderFrame(),
		BaseCSS: testBaseCSS, ThemeID: "swiss-modern", ThemeCSS: testThemeCSS,
	})
	cancel()
	if timeoutErr == nil {
		t.Fatal("render timeout was not enforced")
	}
	postCancelScreenshot := filepath.Join(dir, "post-cancel.png")
	if _, err := renderer.Render(context.Background(), RenderRequest{RuntimeAssetsDir: testRuntimeAssets(t),
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: validToolHTML,
		ScreenshotPath: postCancelScreenshot, ViewportWidth: 1920, ViewportHeight: 1080, TimeoutMS: 15000, Frame: testRenderFrame(),
		BaseCSS: testBaseCSS, ThemeID: "swiss-modern", ThemeCSS: testThemeCSS,
	}); err != nil {
		t.Fatalf("shared browser worker was not usable after canceling one render request: %v", err)
	}
	renderer.mu.Lock()
	postCancelPID := renderer.cmd.Process.Pid
	renderer.mu.Unlock()
	if postCancelPID != initialPID {
		t.Fatalf("canceling one render closed the shared worker: first=%d after_cancel=%d", initialPID, postCancelPID)
	}

	renderer.mu.Lock()
	process := renderer.cmd.Process
	renderer.mu.Unlock()
	if err := syscall.Kill(-process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		renderer.mu.Lock()
		stopped := renderer.cmd == nil
		renderer.mu.Unlock()
		if stopped {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	recoveryScreenshot := filepath.Join(dir, "shot-recovered.png")
	if _, err := renderer.Render(context.Background(), RenderRequest{RuntimeAssetsDir: testRuntimeAssets(t),
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: validToolHTML,
		ScreenshotPath: recoveryScreenshot, ViewportWidth: 1920, ViewportHeight: 1080, TimeoutMS: 15000, Frame: testRenderFrame(),
		BaseCSS: testBaseCSS, ThemeID: "swiss-modern", ThemeCSS: testThemeCSS,
	}); err != nil {
		t.Fatalf("worker did not restart after crash: %v", err)
	}
}

func testRuntimeAssets(t *testing.T) string {
	t.Helper()
	dir, err := runtimeassets.RenderAssetDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
