package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/branding"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

var errScreenshotInvalid = errors.New("cannot establish a current screenshot")
var errScreenshotPageMissing = errors.New("screenshot page is absent from the current outline")

type imagePage struct {
	ID           string
	Ordinal      int
	SectionTitle json.RawMessage
}

// Read only page identities/order and potentially visible section titles. The
// authoring Schema, page titles and Spec are not pixel dependencies.
func imagePages(pack contextengine.ContextPack, root string, session *RunSession) ([]imagePage, error) {
	raw, _, err := readArtifact(root, session, outlineRef(pack))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, pptmutation.ErrOutlineNotExist
	}
	if err != nil {
		return nil, imageSourceReadError("outline", err)
	}
	type page struct {
		ID string `json:"id"`
	}
	var outline struct {
		Sections []struct {
			Title       json.RawMessage `json:"title"`
			Slides      []page          `json:"slides"`
			Subsections []struct {
				Slides []page `json:"slides"`
			} `json:"subsections"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(raw, &outline); err != nil {
		return nil, fmt.Errorf("%w: outline page locations cannot be parsed: %v", errScreenshotInvalid, err)
	}
	if outline.Sections == nil {
		return nil, fmt.Errorf("%w: outline sections must be an array", errScreenshotInvalid)
	}
	pages := []imagePage{}
	seen := map[string]bool{}
	appendPages := func(list []page, title json.RawMessage) error {
		if list == nil {
			return fmt.Errorf("%w: outline slides must be arrays", errScreenshotInvalid)
		}
		for _, node := range list {
			if !model.IsSlideID(node.ID) || seen[node.ID] {
				return fmt.Errorf("%w: outline page ID %q is invalid or ambiguous", errScreenshotInvalid, node.ID)
			}
			seen[node.ID] = true
			pages = append(pages, imagePage{ID: node.ID, Ordinal: len(pages) + 1, SectionTitle: title})
		}
		return nil
	}
	for _, section := range outline.Sections {
		if section.Subsections == nil {
			return nil, fmt.Errorf("%w: outline subsections must be arrays", errScreenshotInvalid)
		}
		if err := appendPages(section.Slides, section.Title); err != nil {
			return nil, err
		}
		for _, sub := range section.Subsections {
			if err := appendPages(sub.Slides, section.Title); err != nil {
				return nil, err
			}
		}
	}
	return pages, nil
}

func imageSourceReadError(source string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: required %s source is missing: %w", errScreenshotInvalid, source, err)
	}
	return fmt.Errorf("read screenshot %s dependency: %w", source, err)
}

func imagePageLocation(pack contextengine.ContextPack, root string, session *RunSession, slideID string) (*imagePage, error) {
	pages, err := imagePages(pack, root, session)
	if err != nil {
		return nil, err
	}
	for i := range pages {
		if pages[i].ID == slideID {
			return &pages[i], nil
		}
	}
	return nil, errScreenshotPageMissing
}

// Keep pixel freshness separate from currentRenderProof's full authoring and
// review evidence. The hash format is shared with renders, without Spec/Design.
func currentImageDependencyHash(pack contextengine.ContextPack, root string, session *RunSession, entry renderimage.Entry) (string, error) {
	location, err := imagePageLocation(pack, root, session, entry.SlideID)
	if err != nil {
		return "", err
	}
	html, _, err := readArtifact(root, session, slideHTMLRef(entry.SlideID))
	if err != nil {
		return "", imageSourceReadError("HTML", err)
	}
	if hashBytes(html) != entry.SourceHash {
		return "", fmt.Errorf("%w: HTML changed since the screenshot was rendered", errScreenshotInvalid)
	}
	frame := spec.RuntimeFrameContext{Canvas: spec.CanonicalCanvas(), Decorations: pack.Project.Decorations, Ordinal: location.Ordinal}
	if frame.Decorations.SectionTitle != "none" {
		var title *string
		if err := json.Unmarshal(location.SectionTitle, &title); err != nil || title == nil {
			return "", fmt.Errorf("%w: the visible section title is not available as text", errScreenshotInvalid)
		}
		frame.Section.Title = *title
	}
	if frame.Decorations.DeckTitle != "none" {
		raw, _, err := readArtifact(root, session, manifestRef(pack))
		if errors.Is(err, fs.ErrNotExist) {
			// The renderer uses this same project title for an uncreated manifest.
			frame.DeckTitle = pack.Project.Title
		} else if err != nil {
			return "", imageSourceReadError("manifest title", err)
		} else {
			var manifest struct {
				Title *string `json:"title"`
			}
			if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Title == nil {
				return "", fmt.Errorf("%w: the visible manifest title cannot be parsed", errScreenshotInvalid)
			}
			frame.DeckTitle = *manifest.Title
		}
	}
	frame.Appearance, err = runtimeassets.ProjectAppearance(root, pack.Project.ThemeID)
	if err != nil {
		if errors.Is(err, runtimeassets.ErrAppearanceInvalid) || errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: page appearance is unavailable: %v", errScreenshotInvalid, err)
		}
		return "", err
	}
	if frame.Decorations.Logo != "none" {
		frame.Logo, err = branding.RenderImage(root, pack.Project.Logo)
		if err != nil {
			if errors.Is(err, branding.ErrInvalid) || errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("%w: visible logo is unavailable: %v", errScreenshotInvalid, err)
			}
			return "", err
		}
	}
	resourceHash, err := renderimage.ResourceHash(root, entry.ResourceHashes)
	if err != nil {
		return "", err
	}
	return (RenderProof{ArtifactHash: entry.SourceHash, FrameContextHash: spec.RuntimeFrameHash(frame), ResourceHash: resourceHash, RendererVersion: renderValidationVersion}).DependencyHash(), nil
}
