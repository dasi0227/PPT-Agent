package workflow

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type RenderedImageContext struct {
	SlideID    string `json:"slide_id"`
	ImagePath  string `json:"image_path"`
	Revision   int    `json:"revision"`
	SourceHash string `json:"source_hash"`
	RenderedAt int64  `json:"rendered_at"`
	Stale      bool   `json:"stale"`
}

func latestRenderedImages(pack contextengine.ContextPack, root string, session *RunSession) []RenderedImageContext {
	outline, err := (contextengine.OutlineLoader{}).Load(root)
	if err != nil {
		return nil
	}
	images := []RenderedImageContext{}
	for _, location := range spec.FlattenOutline(outline) {
		entry, err := renderimage.Latest(root, pack.Project.ID, location.Slide.SlideID)
		if err != nil {
			continue
		}
		proof, proofErr := currentMaterializationProof(pack, root, session, entry.SlideID, entry.SourceHash)
		images = append(images, RenderedImageContext{
			SlideID: entry.SlideID, ImagePath: entry.ImagePath, Revision: entry.Revision,
			SourceHash: entry.SourceHash, RenderedAt: entry.RenderedAt,
			Stale: proofErr != nil || entry.DependencyHash != proof.SourceHash+":"+proof.FrameContextHash,
		})
	}
	return images
}
