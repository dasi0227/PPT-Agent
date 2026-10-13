package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/branding"
	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

type appearanceStore struct {
	store.Store
	project model.Project
	failure error
}

func (s *appearanceStore) GetProject(context.Context, string) (model.Project, error) {
	return s.project, nil
}
func (s *appearanceStore) HasActiveRun(context.Context, string) (bool, error) { return false, nil }
func (s *appearanceStore) HasActiveGitCommit(context.Context, string) (bool, error) {
	return false, nil
}
func (s *appearanceStore) UpdateProjectLogo(_ context.Context, _ string, logo *decoration.LogoMetadata, _ int64) error {
	if s.failure != nil {
		return s.failure
	}
	s.project.Logo = logo
	return nil
}

func TestLogoReplacementKeepsOneFileAndRollsBackRejectedPersistence(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st := &appearanceStore{project: model.Project{ID: "pro_logo", WorkDir: filepath.Join(root, "artifacts"), Decorations: decoration.DefaultPositions()}}
	if err := os.MkdirAll(st.project.WorkDir, 0755); err != nil {
		t.Fatal(err)
	}
	svc := NewProjectService(st, WorkRoot(root))
	picture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var pngBytes, jpegBytes bytes.Buffer
	if err := png.Encode(&pngBytes, picture); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegBytes, picture, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UploadLogo(ctx, st.project.ID, "first.png", bytes.NewReader(pngBytes.Bytes())); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UploadLogo(ctx, st.project.ID, "second.jpg", bytes.NewReader(jpegBytes.Bytes())); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "branding"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "logo.jpg" {
		t.Fatalf("logo files: %v (%v)", entries, err)
	}
	previous := st.project.Logo.Hash
	st.failure = errors.New("persistence refused")
	if _, err := svc.UploadLogo(ctx, st.project.ID, "third.png", bytes.NewReader(pngBytes.Bytes())); !errors.Is(err, st.failure) {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, err := branding.Read(st.project.WorkDir, st.project.Logo)
	if err != nil || !bytes.Equal(raw, jpegBytes.Bytes()) || st.project.Logo.Hash != previous {
		t.Fatalf("previous logo lost: %v", err)
	}
	entries, err = os.ReadDir(filepath.Join(root, "branding"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "logo.jpg" {
		t.Fatalf("failed upload left files: %v (%v)", entries, err)
	}
	if _, err := svc.DeleteLogo(ctx, st.project.ID); !errors.Is(err, st.failure) {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if _, err := branding.Read(st.project.WorkDir, st.project.Logo); err != nil {
		t.Fatal("failed delete lost the image", err)
	}
	st.failure = nil
	if _, err := svc.DeleteLogo(ctx, st.project.ID); err != nil {
		t.Fatal(err)
	}
	if st.project.Logo != nil {
		t.Fatal("deleted image still referenced")
	}
	entries, err = os.ReadDir(filepath.Join(root, "branding"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("deleted logo left files: %v (%v)", entries, err)
	}
}
