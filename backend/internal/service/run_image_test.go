package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunImageResolverRejectsCrossRunAndCrossProjectReferences(t *testing.T) {
	currentProject := t.TempDir()
	otherProject := t.TempDir()
	ref := "run:run-current/screenshot:shot_abc"
	relative := filepath.Join(".runtime", "renders", "run-current", "shot_abc.png")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(otherProject, relative)), 0o700); err != nil {
		t.Fatal(err)
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("runtime-owned-screenshot")...)
	if err := os.WriteFile(filepath.Join(otherProject, relative), png, 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := runImageResolver{runID: "run-current", projectID: "project-current", projectDir: currentProject}
	if _, err := resolver.ResolveImage(context.Background(), "run:run-other/screenshot:shot_abc"); err == nil {
		t.Fatal("cross-run image reference was accepted")
	}
	if _, err := resolver.ResolveImage(context.Background(), ref); err == nil {
		t.Fatal("screenshot from another project directory was accepted")
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(currentProject, relative)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(currentProject, relative), png, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := resolver.ResolveImage(context.Background(), ref)
	if err != nil || image.MIMEType != "image/png" || string(image.Bytes) != string(png) {
		t.Fatalf("current run screenshot was not resolved: image=%+v err=%v", image, err)
	}
}
