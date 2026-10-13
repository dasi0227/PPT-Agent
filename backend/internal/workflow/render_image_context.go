package workflow

import (
	"errors"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

type RenderedImageContext struct {
	SlideID    string `json:"slide_id"`
	ImagePath  string `json:"image_path"`
	SourceHash string `json:"source_hash"`
	RenderedAt int64  `json:"rendered_at"`
	Stale      bool   `json:"stale"`
}

func latestRenderedImages(pack contextengine.ContextPack, root string, session *RunSession) ([]RenderedImageContext, error) {
	pages, err := imagePages(pack, root, session)
	if errors.Is(err, os.ErrNotExist) {
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
		if dependencyErr != nil && !errors.Is(dependencyErr, errScreenshotInvalid) && !errors.Is(dependencyErr, errScreenshotPageMissing) {
			return nil, dependencyErr
		}
		images = append(images, RenderedImageContext{
			SlideID: entry.SlideID, ImagePath: entry.ImagePath(),
			SourceHash: entry.SourceHash, RenderedAt: entry.RenderedAt,
			Stale: dependencyErr != nil || entry.DependencyHash == "" || entry.DependencyHash != dependencyHash,
		})
	}
	return images, nil
}
