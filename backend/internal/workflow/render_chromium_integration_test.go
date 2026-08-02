package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

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
		ScreenshotPath: screenshot, ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.ScreenshotBytes <= 0 || diagnostics.FontStatus != "loaded" {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	if _, err := os.Stat(screenshot); err != nil {
		t.Fatal(err)
	}

	secondScreenshot := filepath.Join(dir, "shot-2.png")
	secondHTML := `<!doctype html><html><body><section class="slide-stage"><h1>Second</h1><script>if(localStorage.getItem('leak'))console.error('context leaked')</script></section></body></html>`
	diagnostics, err = renderer.Render(context.Background(), RenderRequest{
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: secondHTML,
		ScreenshotPath: secondScreenshot, ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000,
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
	stagingDir := filepath.Join(dir, ".staging", "integration")
	if err := os.MkdirAll(filepath.Join(stagingDir, "common"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "common", "tokens.css"), []byte(":root{--overlay-proof: 37px}"), 0o644); err != nil {
		t.Fatal(err)
	}
	overlayHTML := `<!doctype html><html><head><link rel="stylesheet" href="../../common/tokens.css"></head><body><section class="slide-stage"><div id="proof" style="width:var(--overlay-proof)">Overlay</div></section><script>if(getComputedStyle(document.querySelector('#proof')).width!=='37px')console.error('staging overlay missing')</script></body></html>`
	overlayDiagnostics, overlayErr := renderer.Render(context.Background(), RenderRequest{
		RunID: "integration", ProjectDir: dir, StagingDir: stagingDir, SlideID: "slide-01", HTML: overlayHTML,
		ScreenshotPath: filepath.Join(dir, "overlay.png"), ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000,
	})
	if overlayErr != nil || len(overlayDiagnostics.ConsoleErrors) != 0 {
		t.Fatalf("staging overlay was not served: diagnostics=%+v err=%v", overlayDiagnostics, overlayErr)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, timeoutErr := renderer.Render(timeoutCtx, RenderRequest{
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01",
		HTML:           `<html><body><section class="slide-stage"><script>while(true){}</script></section></body></html>`,
		ScreenshotPath: filepath.Join(dir, "timeout.png"), ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 1000,
	})
	cancel()
	if timeoutErr == nil {
		t.Fatal("render timeout was not enforced")
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
		ScreenshotPath: recoveryScreenshot, ViewportWidth: 1600, ViewportHeight: 900, TimeoutMS: 15000,
	}); err != nil {
		t.Fatalf("worker did not restart after crash: %v", err)
	}
}
