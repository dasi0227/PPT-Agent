package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const validToolHTML = `<!doctype html><html lang="zh"><head><link rel="stylesheet" href="../../common/tokens.css"><link rel="stylesheet" href="../../common/base.css"></head><body><section class="slide-stage"><h1>Original</h1></section></body></html>`

func testRenderFrame() spec.RuntimeFrameContext {
	return spec.RuntimeFrameContext{SlideID: "slide-01", DeckTitle: "Deck", Ordinal: 2, Total: 2, Role: "content", Section: spec.RuntimeFrameAncestor{ID: "sec_test", Title: "Section", Index: 1}, Numbering: spec.RuntimeFrameNumbering{Visible: true, Format: "number"}, Chrome: []spec.ChromeItem{{Type: "page_number", Placement: "bottom-right", Style: "muted"}, {Type: "section_marker", Placement: "top-left", Style: "muted"}, {Type: "deck_title", Placement: "top-right", Style: "muted"}}}
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
	for _, rel := range []string{"slides/slide-01", "common"} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "common", "tokens.css"), []byte(":root{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "common", "base.css"), []byte(".slide-stage{width:1600px;height:900px;overflow:hidden}"), 0o644); err != nil {
		t.Fatal(err)
	}
	screenshot := filepath.Join(dir, "shot.png")
	diagnostics, err := renderer.Render(context.Background(), RenderRequest{
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01",
		HTML:           strings.Replace(validToolHTML, "</body>", `<script>localStorage.setItem('leak','yes')</script></body>`, 1),
		ScreenshotPath: screenshot, ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000, Frame: testRenderFrame(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.ScreenshotBytes <= 0 || diagnostics.FontStatus != "loaded" {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	if strings.Join(diagnostics.RuntimeChrome, ",") != "page_number,section_marker,deck_title" {
		t.Fatalf("runtime chrome was not injected: %#v", diagnostics.RuntimeChrome)
	}
	if _, err := os.Stat(screenshot); err != nil {
		t.Fatal(err)
	}

	secondScreenshot := filepath.Join(dir, "shot-2.png")
	secondHTML := `<!doctype html><html><body><section class="slide-stage"><h1>Second</h1><script>if(localStorage.getItem('leak'))console.error('context leaked')</script></section></body></html>`
	diagnostics, err = renderer.Render(context.Background(), RenderRequest{
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: secondHTML,
		ScreenshotPath: secondScreenshot, ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000, Frame: testRenderFrame(),
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
	if err := os.WriteFile(filepath.Join(dir, "common", "tokens.css"), []byte(":root{--project-proof: 37px}"), 0o644); err != nil {
		t.Fatal(err)
	}
	projectAssetHTML := `<!doctype html><html><head><link rel="stylesheet" href="../../common/tokens.css"></head><body><section class="slide-stage"><div id="proof" style="width:var(--project-proof)">Project asset</div></section><script>if(getComputedStyle(document.querySelector('#proof')).width!=='37px')console.error('project asset missing')</script></body></html>`
	projectAssetDiagnostics, projectAssetErr := renderer.Render(context.Background(), RenderRequest{
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: projectAssetHTML,
		ScreenshotPath: filepath.Join(dir, "overlay.png"), ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000, Frame: testRenderFrame(),
	})
	if projectAssetErr != nil || len(projectAssetDiagnostics.ConsoleErrors) != 0 {
		t.Fatalf("project asset was not served: diagnostics=%+v err=%v", projectAssetDiagnostics, projectAssetErr)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, timeoutErr := renderer.Render(timeoutCtx, RenderRequest{
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01",
		HTML:           `<html><body><section class="slide-stage"><script>while(true){}</script></section></body></html>`,
		ScreenshotPath: filepath.Join(dir, "timeout.png"), ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 1000, Frame: testRenderFrame(),
	})
	cancel()
	if timeoutErr == nil {
		t.Fatal("render timeout was not enforced")
	}
	postCancelScreenshot := filepath.Join(dir, "post-cancel.png")
	if _, err := renderer.Render(context.Background(), RenderRequest{
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: validToolHTML,
		ScreenshotPath: postCancelScreenshot, ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000, Frame: testRenderFrame(),
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
	if _, err := renderer.Render(context.Background(), RenderRequest{
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: validToolHTML,
		ScreenshotPath: recoveryScreenshot, ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000, Frame: testRenderFrame(),
	}); err != nil {
		t.Fatalf("worker did not restart after crash: %v", err)
	}
}
