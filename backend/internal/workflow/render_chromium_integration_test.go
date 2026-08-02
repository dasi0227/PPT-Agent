package workflow

import (
	"context"
	"os"
	"path/filepath"
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
	if err := renderer.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
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
		RunID: "integration", ProjectDir: dir, SlideID: "slide-01", HTML: validToolHTML,
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
}
