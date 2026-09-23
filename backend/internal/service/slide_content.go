package service

import (
	"context"
	"errors"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

var ErrSlideHTMLMissing = errors.New("service: slide html not rendered")
var ErrSlideHTMLChanged = errors.New("service: slide html changed")

func (svc *SlideService) ReadHTML(ctx context.Context, slideID, expectedHash string) ([]byte, error) {
	slide, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return nil, err
	}
	project, err := svc.store.GetProject(ctx, slide.ProjectID)
	if err != nil {
		return nil, err
	}
	sandbox, err := artifactfs.NewSandbox(project.WorkDir)
	if err != nil {
		return nil, err
	}
	raw, err := sandbox.Read(model.SlideHTMLPath(slideID))
	if os.IsNotExist(err) {
		return nil, ErrSlideHTMLMissing
	}
	if err != nil {
		return nil, err
	}
	// HTML identity is independent of appearance. Runtime adds resources when
	// executing this document; a theme change must not replace its iframe.
	if expectedHash != "" && spec.ContentHash(raw) != expectedHash {
		return nil, ErrSlideHTMLChanged
	}
	return raw, nil
}
