package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

var stableSlideID = regexp.MustCompile(`^sli_[A-Za-z0-9_-]+$`)

func manifestRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactManifest, ID: pack.Project.ID, Path: ".manifest.json", Project: pack.Project.ID}
}
func outlineRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactOutline, ID: pack.Project.ID, Path: ".outline.json", Project: pack.Project.ID}
}
func designRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactDesign, ID: pack.Project.ID, Path: ".design.json", Project: pack.Project.ID}
}
func specSlideRef(id string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactSlideSpec, ID: id, Path: model.SpecCollectionPath}
}
func slideHTMLRef(id string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactSlideHTML, ID: id, Path: model.SlideHTMLPath(id)}
}
func refForResource(pack contextengine.ContextPack, r Resource) (ArtifactRef, error) {
	switch r.Key() {
	case "deck:manifest":
		return manifestRef(pack), nil
	case "deck:outline":
		return outlineRef(pack), nil
	case "deck:design":
		return designRef(pack), nil
	}
	if r.Type == "slide" && r.Part == "spec" {
		return specSlideRef(r.SlideID), nil
	}
	if r.Type == "slide" && r.Part == "html" {
		return slideHTMLRef(r.SlideID), nil
	}
	return ArtifactRef{}, errors.New("unsupported PPT resource")
}
func parseResource(args map[string]any) (Resource, error) {
	kind, ok := args["resource"].(string)
	if !ok {
		return Resource{}, errors.New("resource must be a resource name")
	}
	id := stringValue(args["slide_id"])
	switch kind {
	case "manifest", "design", "outline":
		if _, present := args["slide_id"]; present {
			return Resource{}, errors.New("global resources do not accept slide_id")
		}
		return Resource{Type: "deck", Part: kind}, nil
	case "spec", "html":
		if !stableSlideID.MatchString(id) {
			return Resource{}, errors.New("spec and html require a stable slide_id")
		}
		return Resource{Type: "slide", Part: kind, SlideID: id}, nil
	}
	return Resource{}, errors.New("unknown resource")
}
func readArtifact(projectDir string, tx *RunSession, ref ArtifactRef) ([]byte, string, error) {
	if tx != nil {
		raw, err := tx.Read(ref)
		source := "committed"
		if tx.HasChange(ref) {
			source = "run_overlay"
		}
		return raw, source, err
	}
	raw, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(ref.Path)))
	if err == nil && ref.Kind == ArtifactSlideSpec {
		raw, err = spec.CollectionEntry(raw, ref.ID)
	}
	return raw, "committed", err
}
func errorsIsNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }
func resourceSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"manifest", "design", "outline", "spec", "html"}}
}
func validateHTML(raw []byte) ([]Issue, error) {
	if err := spec.ValidateSlideHTML(raw); err != nil {
		code := "HTML_PARSE"
		if errors.Is(err, spec.ErrSlideStageMissing) {
			code = "stage-16-9"
		}
		if errors.Is(err, spec.ErrStaticPageNumber) {
			code = "STATIC_PAGE_NUMBER"
		}
		return []Issue{{Code: code, Severity: SeverityError, Summary: err.Error()}}, err
	}
	return []Issue{}, nil
}
func currentManifest(pack contextengine.ContextPack, tx *RunSession) (spec.Manifest, error) {
	raw, _, err := readArtifact(tx.ProjectDir(), tx, manifestRef(pack))
	if err != nil {
		return spec.Manifest{}, err
	}
	var value spec.Manifest
	err = json.Unmarshal(raw, &value)
	return value, err
}
func currentOutline(pack contextengine.ContextPack, tx *RunSession) (spec.Outline, error) {
	raw, _, err := readArtifact(tx.ProjectDir(), tx, outlineRef(pack))
	if errors.Is(err, fs.ErrNotExist) {
		return spec.Outline{Sections: []spec.Section{}}, nil
	}
	if err != nil {
		return spec.Outline{}, err
	}
	var value spec.Outline
	err = json.Unmarshal(raw, &value)
	return value, err
}
func validateReferences(pack contextengine.ContextPack, tx *RunSession) (string, error) {
	outline, err := currentOutline(pack, tx)
	if err != nil {
		return "", err
	}
	if err = spec.ValidateOutline(outline); err != nil {
		return "", err
	}
	combined := []byte{}
	for _, loc := range spec.FlattenOutline(outline) {
		raw, _, readErr := readArtifact(tx.ProjectDir(), tx, specSlideRef(loc.Slide.SlideID))
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return "", readErr
		}
		var slide spec.SlideSpec
		if json.Unmarshal(raw, &slide) != nil || spec.ValidateSlideSpec(slide) != nil {
			return "", fmt.Errorf("invalid spec for %s", loc.Slide.SlideID)
		}
		combined = append(combined, raw...)
	}
	return hashBytes(combined), nil
}
func targetHash(pack contextengine.ContextPack, tx *RunSession, target Resource) (string, error) {
	ref, err := refForResource(pack, target)
	if err != nil {
		return "", err
	}
	raw, _, err := readArtifact(tx.ProjectDir(), tx, ref)
	if err != nil {
		return "", err
	}
	return hashBytes(raw), nil
}

func currentRenderProof(pack contextengine.ContextPack, projectDir string, tx *RunSession, slideID, artifactHash string) (RenderProof, error) {
	deckRaw, _, err := readArtifact(projectDir, tx, manifestRef(pack))
	if err != nil {
		return RenderProof{}, err
	}
	outlineRaw, _, err := readArtifact(projectDir, tx, outlineRef(pack))
	if err != nil {
		return RenderProof{}, err
	}
	designRaw, _, err := readArtifact(projectDir, tx, designRef(pack))
	if err != nil {
		return RenderProof{}, err
	}
	specRaw, _, err := readArtifact(projectDir, tx, specSlideRef(slideID))
	if err != nil {
		return RenderProof{}, err
	}
	htmlRaw, _, err := readArtifact(projectDir, tx, slideHTMLRef(slideID))
	if err != nil {
		return RenderProof{}, err
	}
	if hashBytes(htmlRaw) != artifactHash {
		return RenderProof{}, errors.New("rendered HTML hash is stale")
	}
	var deck spec.Manifest
	var outline spec.Outline
	var design spec.Design
	var slide spec.SlideSpec
	if json.Unmarshal(deckRaw, &deck) != nil || json.Unmarshal(outlineRaw, &outline) != nil || json.Unmarshal(designRaw, &design) != nil || json.Unmarshal(specRaw, &slide) != nil {
		return RenderProof{}, errors.New("render source is invalid")
	}
	appearance, err := runtimeassets.ProjectAppearance(projectDir, pack.Project.ThemeID)
	if err != nil {
		return RenderProof{}, err
	}
	nodeHash := spec.SemanticSlideNodeHash(outline, slideID)
	return RenderProof{SlideID: slideID, ManifestHash: spec.ResourceHash(deck), OutlineNodeHash: nodeHash, SpecHash: spec.ResourceHash(slide), DesignContentHash: spec.DesignContentHash(design), ArtifactHash: artifactHash, SourceHash: spec.SourceHash(deckRaw, nodeHash, specRaw, designRaw), FrameContextHash: spec.FrameContextHash(deck, outline, design, slideID, slide, appearance)}, nil
}
func RenderSourceHash(deckRaw []byte, nodeHash string, specRaw, designRaw []byte) string {
	return spec.SourceHash(deckRaw, nodeHash, specRaw, designRaw)
}
func newEvidence(kind string, target Resource, sourceHash string, values ...map[string]any) Evidence {
	data := map[string]any{}
	if len(values) > 0 && values[0] != nil {
		data = values[0]
	}
	return Evidence{ID: fmt.Sprintf("evidence_%d_%s", time.Now().UnixNano(), kind), Kind: kind, Target: target, SourceHash: sourceHash, ProducedAt: time.Now().Unix(), Data: data}
}
func uniqueTargets(values []Resource) []Resource {
	out := []Resource{}
	seen := map[string]bool{}
	for _, v := range values {
		if v.Type != "" && !seen[v.Key()] {
			seen[v.Key()] = true
			out = append(out, v)
		}
	}
	return out
}

func lineDiffStat(before, after []byte) (int, int) {
	a := strings.Split(strings.TrimSpace(string(before)), "\n")
	b := strings.Split(strings.TrimSpace(string(after)), "\n")
	if len(before) == 0 {
		return len(b), 0
	}
	if len(after) == 0 {
		return 0, len(a)
	}
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				current[j] = previous[j-1] + 1
			} else if previous[j] > current[j-1] {
				current[j] = previous[j]
			} else {
				current[j] = current[j-1]
			}
		}
		previous, current = current, previous
		for j := range current {
			current[j] = 0
		}
	}
	common := previous[len(b)]
	return len(b) - common, len(a) - common
}
