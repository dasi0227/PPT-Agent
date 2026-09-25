package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

func TestRunImageResolverReadsOnlyIndexedProjectRenders(t *testing.T) {
	currentProject := t.TempDir()
	otherProject := t.TempDir()
	entry := renderimage.Entry{ProjectID: "project-current", SlideID: "sli_one", RunID: "run-current", ScreenshotID: "shot_abc", SourceHash: "html-hash"}
	ref := entry.ImageRef()
	relative := filepath.Join(".runtime", "renders", "run-current", "shot_abc.png")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(otherProject, relative)), 0o700); err != nil {
		t.Fatal(err)
	}
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherProject, relative), imageBytes.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := runImageResolver{projectID: "project-current", projectDir: currentProject}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolver.ResolveImage(canceled, ref); err != context.Canceled {
		t.Fatalf("image resolver ignored context cancellation: %v", err)
	}
	if _, err := resolver.ResolveImage(context.Background(), "run:run-other/screenshot:shot_abc"); err == nil {
		t.Fatal("cross-run image reference was accepted")
	}
	if _, err := resolver.ResolveImage(context.Background(), ref); err == nil {
		t.Fatal("screenshot from another project directory was accepted")
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(currentProject, relative)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(currentProject, relative), imageBytes.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveImage(context.Background(), ref); err == nil {
		t.Fatal("unindexed screenshot accepted")
	}
	if err := renderimage.Publish(currentProject, entry); err != nil {
		t.Fatal(err)
	}
	image, err := resolver.ResolveImage(context.Background(), ref)
	if err != nil || image.MIMEType != "image/png" || !bytes.Equal(image.Bytes, imageBytes.Bytes()) {
		t.Fatalf("indexed project screenshot was not resolved: image=%+v err=%v", image, err)
	}
	if _, err := resolver.ResolveImage(context.Background(), "project:project-other/render:sli_one/shot_abc"); err == nil {
		t.Fatal("cross-project reference accepted")
	}
}
