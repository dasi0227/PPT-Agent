package workflow

import (
	"errors"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

type RenderedImageContext struct {
	SlideID        string `json:"slide_id"`
	ImagePath      string `json:"image_path"`
	ImageRef       string `json:"image_ref"`
	SourceHash     string `json:"source_hash"`
	DependencyHash string `json:"dependency_hash"`
	RenderedAt     int64  `json:"rendered_at"`
	Stale          bool   `json:"stale"`
}

func latestRenderedImages(pack contextengine.ContextPack, root string, session *RunSession) ([]RenderedImageContext, error) {
	pages, err := imagePages(pack, root, session)
	if errors.Is(err, pptmutation.ErrOutlineNotExist) {
		// A new project has no outline or screenshot context yet.
		return []RenderedImageContext{}, nil
	}
	if err != nil {
		return nil, err
	}
	images := []RenderedImageContext{}
	for _, page := range pages {
		entry, err := renderimage.Latest(root, pack.Project.ID, page.ID)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if errors.Is(err, renderimage.ErrUnavailable) {
			continue
		}
		if err != nil {
			return nil, err
		}
		dependencyHash, dependencyErr := currentImageDependencyHash(pack, root, session, entry)
		// The outline can disappear between listing pages and reading dependencies.
		if dependencyErr != nil &&
			!errors.Is(dependencyErr, pptmutation.ErrOutlineNotExist) &&
			!errors.Is(dependencyErr, errScreenshotInvalid) &&
			!errors.Is(dependencyErr, errScreenshotPageMissing) {
			return nil, dependencyErr
		}
		images = append(images, RenderedImageContext{
			SlideID: entry.SlideID, ImagePath: entry.ImagePath(),
			ImageRef: entry.ImageRef(), SourceHash: entry.SourceHash, DependencyHash: entry.DependencyHash, RenderedAt: entry.RenderedAt,
			Stale: dependencyErr != nil || entry.DependencyHash == "" || entry.DependencyHash != dependencyHash,
		})
	}
	return images, nil
}
