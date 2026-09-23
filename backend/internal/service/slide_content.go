package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimehtml"
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
	// Check the same source bytes that will be normalized and returned. The
	// normalized preview itself is not the authored HTML's content identity.
	if expectedHash != "" && spec.ContentHash(raw) != expectedHash {
		return nil, ErrSlideHTMLChanged
	}
	designRaw, err := sandbox.Read("design.json")
	if err != nil {
		return nil, err
	}
	var design spec.Design
	if err := json.Unmarshal(designRaw, &design); err != nil {
		return nil, err
	}
	return runtimehtml.Normalize(raw, design.Theme)
}
