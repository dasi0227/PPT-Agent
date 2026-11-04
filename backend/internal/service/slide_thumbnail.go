package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/branding"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimehtml"
	"github.com/dasi0227/PPT-Agent/backend/internal/slidepreview"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type thumbnailInput struct {
	project  model.Project
	html     []byte
	frame    spec.RuntimeFrameContext
	key      string
	version  string
	themeCSS []byte
}

// The caller holds the project read lock; rendering itself never holds an authoring lock.
func (svc *SlideService) thumbnailInput(ctx context.Context, project model.Project, slideID string) (thumbnailInput, error) {
	var input thumbnailInput
	project, err := svc.store.GetProject(ctx, project.ID)
	if err != nil {
		return input, err
	}
	sandbox, err := artifactfs.NewSandbox(project.WorkDir)
	if err != nil {
		return input, err
	}
	html, err := sandbox.Read(model.SlideHTMLPath(slideID))
	if err != nil {
		return input, err
	}
	outlineRaw, err := sandbox.Read(".outline.json")
	if err != nil {
		return input, err
	}
	parsed, err := spec.ParseStrictSourceJSON(outlineRaw, "outline")
	if err != nil {
		return input, err
	}
	outline := *parsed.(*spec.Outline)
	meta := spec.Meta{Title: project.Title}
	if raw, err := sandbox.Read(".meta.json"); err == nil {
		parsed, err := spec.ParseStrictSourceJSON(raw, "meta")
		if err != nil {
			return input, err
		}
		meta = *parsed.(*spec.Meta)
	} else if !errors.Is(err, os.ErrNotExist) {
		return input, err
	}
	var slide spec.SlideSpec
	if raw, err := spec.ReadSlideSpec(sandbox.Read, slideID); err == nil {
		if err := json.Unmarshal(raw, &slide); err != nil {
			return input, err
		}
	} else if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, spec.ErrInvalid) {
		return input, err
	}
	themeCSS, err := svc.themes.CSS(project.Theme)
	if err != nil {
		return input, err
	}
	logo, err := branding.RenderImage(project.WorkDir, project.Logo)
	if err != nil {
		return input, err
	}
	frame, ok := spec.BuildRuntimeFrame(meta, outline, project.Decorations, logo, slideID, slide, runtimeassets.Appearance(project.Theme, themeCSS))
	if !ok {
		return input, ErrSlideHTMLMissing
	}
	key := slidepreview.SourceKey(spec.ContentHash(html), frame)
	return thumbnailInput{project: project, html: html, frame: frame, key: key,
		version: slidepreview.Version(project.WorkDir, slideID, key), themeCSS: themeCSS}, nil
}

func (svc *SlideService) Thumbnail(ctx context.Context, slideID, expectedVersion string, generate bool) ([]byte, error) {
	slide, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return nil, err
	}
	project, err := svc.store.GetProject(ctx, slide.ProjectID)
	if err != nil {
		return nil, err
	}
	read := func() (thumbnailInput, []byte, error) {
		release := pptmutation.ReadLockProject(project.WorkDir)
		defer release()
		input, err := svc.thumbnailInput(ctx, project, slideID)
		if err != nil {
			return input, nil, err
		}
		if expectedVersion != input.version && expectedVersion != slidepreview.InitialVersion(input.key) {
			return input, nil, ErrSlideHTMLChanged
		}
		raw, _ := slidepreview.Read(project.WorkDir, slideID, input.key)
		return input, raw, nil
	}
	_, raw, err := read()
	if err != nil || raw != nil {
		return raw, err
	}
	if !generate {
		return nil, slidepreview.ErrUnavailable
	}
	// One thumbnail job at a time leaves shared render capacity for authoring/export.
	select {
	case svc.thumbnailSlots <- struct{}{}:
		defer func() { <-svc.thumbnailSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	input, raw, err := read()
	if err != nil || raw != nil {
		return raw, err
	}
	if svc.renderer == nil {
		return nil, slidepreview.ErrUnavailable
	}
	normalized, err := runtimehtml.Normalize(input.html, input.project.Theme)
	if err != nil {
		return nil, err
	}
	assetsDir, err := runtimeassets.RenderAssetDir()
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp("", "ppt-thumbnail-*.png")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Close(); err != nil {
		return nil, err
	}
	diagnostics, err := svc.renderer.Render(ctx, workflow.RenderRequest{
		RuntimeAssetsDir: assetsDir, RunID: "thumbnail", ProjectDir: input.project.WorkDir, SlideID: slideID,
		HTML: string(normalized), ScreenshotPath: path, ViewportWidth: 1920, ViewportHeight: 1080,
		TimeoutMS: 15000, Frame: input.frame, BaseCSS: string(runtimeassets.BaseCSS()),
		ThemeID: input.project.Theme, ThemeCSS: string(input.themeCSS),
	})
	if err != nil {
		return nil, err
	}
	if diagnostics.ResourceHashes == nil {
		return nil, slidepreview.ErrUnavailable
	}
	screenshot, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	release := pptmutation.ReadLockProject(project.WorkDir)
	defer release()
	current, err := svc.thumbnailInput(ctx, project, slideID)
	if err != nil {
		return nil, err
	}
	if current.key != input.key {
		return nil, ErrSlideHTMLChanged
	}
	resourceHash, err := renderimage.ResourceHash(project.WorkDir, diagnostics.ResourceHashes)
	if err != nil || resourceHash != renderimage.ResourceSnapshotHash(diagnostics.ResourceHashes) {
		return nil, errors.Join(ErrSlideHTMLChanged, err)
	}
	return slidepreview.Write(project.WorkDir, slideID, input.key, diagnostics.ResourceHashes, screenshot)
}
