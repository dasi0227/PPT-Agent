package contextengine

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

var (
	ErrSourceInvalid   = errors.New("CONTEXT_SOURCE_INVALID")
	ErrRequiredMissing = errors.New("CONTEXT_REQUIRED_MISSING")
)

type ComponentIndexLoader interface {
	LoadComponents(context.Context) ([]model.Component, error)
}

type SkillIndexLoader interface {
	LoadSkills(context.Context) ([]model.RepositorySkill, error)
}

type ContextStore interface {
	GetSlide(context.Context, string) (model.Slide, error)
}

type ContextAssembler struct {
	store      ContextStore
	components ComponentIndexLoader
	skills     SkillIndexLoader
	estimator  TokenEstimator
}

func NewContextAssembler(s ContextStore) *ContextAssembler {
	return &ContextAssembler{store: s, estimator: StableTokenEstimator{}}
}

func (a *ContextAssembler) WithComponentLoader(loader ComponentIndexLoader) *ContextAssembler {
	a.components = loader
	return a
}

func (a *ContextAssembler) WithSkillLoader(loader SkillIndexLoader) *ContextAssembler {
	a.skills = loader
	return a
}

func (a *ContextAssembler) Assemble(ctx context.Context, req ContextRequest, project model.Project) (ContextPack, error) {
	if err := req.Command.Validate(); err != nil {
		return ContextPack{}, err
	}
	return a.AssembleSnapshot(ctx, req, project)
}

// AssembleSnapshot also supports an empty thread, which has no user instruction.
// It only reads context; actual Run creation still requires Command.Validate.
func (a *ContextAssembler) AssembleSnapshot(ctx context.Context, req ContextRequest, project model.Project) (ContextPack, error) {
	if req.ProjectID != project.ID {
		return ContextPack{}, fmt.Errorf("%w: project identity mismatch", ErrRequiredMissing)
	}
	if err := req.Command.Scope.Validate(); err != nil {
		return ContextPack{}, err
	}
	profile := ProfilePPTDeck
	if req.Command.Scope.IsSinglePage() {
		profile = ProfilePPTSlide
	}

	deck, outline, slides, design, err := loadSpec(project)
	if err != nil {
		return ContextPack{}, err
	}
	pack := ContextPack{
		SchemaVersion: SchemaVersion, Profile: profile, Command: req.Command,
		Project:              (ProjectLoader{}).Load(project),
		PresentationManifest: PresentationManifestContext{Manifest: deck},
		Outline:              OutlineContext{Outline: outline, Summaries: []SlideSummary{}},
		Design:               DesignContext{Design: &design},
		SlideHTML:            SlideHTMLContext{Summaries: map[string]HTMLSummary{}},
		Components:           []ComponentCandidate{}, Skills: []SkillCandidate{},
	}
	for _, location := range pptspec.FlattenOutline(outline) {
		id := location.Slide.ID
		s, ready := slides[id]
		summary := slideSummary(location, s, ready)
		if profile == ProfilePPTDeck || profile == ProfilePPTSlide {
			state := loadHTMLState(project.WorkDir, id)
			summary.State = state
		}
		pack.Outline.Summaries = append(pack.Outline.Summaries, summary)
	}
	pack.GenerationInputs = map[string]*pptspec.GenerationInputs{}
	pack.GenerationBaselines = map[string]*pptspec.GenerationInputs{}
	for _, loc := range pptspec.FlattenOutline(outline) {
		id := loc.Slide.ID
		if slide, exists := slides[id]; exists {
			setGenerationInputs(&pack, id, slide)
		}
		if a.store != nil {
			meta, err := a.store.GetSlide(ctx, id)
			if err == nil && meta.ProjectID == project.ID && meta.GenerationInputsJSON != nil {
				pack.GenerationBaselines[id] = pptspec.ParseGenerationInputs([]byte(*meta.GenerationInputsJSON))
			}
		}
	}
	if req.Command.Scope.IsSinglePage() {
		targetID := req.Command.Scope.SlideIDs[0]
		if _, exists := pptspec.FindSlide(outline, targetID); !exists {
			return ContextPack{}, fmt.Errorf("%w: slide %s is not present in outline", ErrRequiredMissing, targetID)
		}
		target, ok := slides[targetID]
		pack.Target = TargetContext{SlideIDs: append([]string{}, req.Command.Scope.SlideIDs...)}
		if ok {
			pack.Target.SlideSpec = &target
		}
	} else {
		pack.Target = TargetContext{SlideIDs: append([]string{}, req.Command.Scope.SlideIDs...)}
	}

	manifest := ContextManifest{
		ContextID: opaqueID("ctx", req.RunID, project.ID, string(profile), string(stableJSON(req.Command))),
		RunID:     req.RunID, ThreadID: req.ThreadID, ProjectID: req.ProjectID,
	}
	a.loadSlideHTML(project, req, &pack, &manifest)

	if a.components != nil {
		components, componentErr := a.components.LoadComponents(ctx)
		if componentErr != nil {
			manifest.Warnings = append(manifest.Warnings, "component repository unavailable: "+componentErr.Error())
		} else {
			pack.Components = componentCandidates(components)
		}
	}
	if a.skills != nil {
		skills, skillErr := a.skills.LoadSkills(ctx)
		if skillErr != nil {
			manifest.Warnings = append(manifest.Warnings, "skill repository unavailable: "+skillErr.Error())
		} else {
			pack.Skills = skillCandidates(skills)
		}
	}
	hashInput := pack
	hashInput.Manifest = ContextManifest{}
	manifest.PackHash = fmt.Sprintf("%x", sha256.Sum256(stableJSON(hashInput)))
	pack.Manifest = manifest
	return pack, nil
}

func loadSpec(project model.Project) (pptspec.Manifest, pptspec.Outline, map[string]pptspec.SlideSpec, pptspec.Design, error) {
	deck, err := (ManifestLoader{}).Load(project.WorkDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return deck, pptspec.Outline{}, nil, pptspec.Design{}, fmt.Errorf("%w: deck: %v", ErrRequiredMissing, err)
	}
	if validationErr := pptspec.ValidateManifest(deck); err == nil && validationErr != nil {
		return deck, pptspec.Outline{}, nil, pptspec.Design{}, fmt.Errorf("%w: %v", ErrSourceInvalid, validationErr)
	}
	outline, err := (OutlineLoader{}).Load(project.WorkDir)
	if err != nil {
		return deck, outline, nil, pptspec.Design{}, fmt.Errorf("%w: outline: %v", ErrRequiredMissing, err)
	}
	ids := []string{}
	for _, loc := range pptspec.FlattenOutline(outline) {
		ids = append(ids, loc.Slide.ID)
	}
	slides, err := (SlideSpecLoader{}).LoadAll(project.WorkDir, ids)
	if err != nil {
		return deck, outline, nil, pptspec.Design{}, fmt.Errorf("%w: %v", ErrRequiredMissing, err)
	}
	if err := pptspec.ValidateOutline(outline); err != nil {
		return deck, outline, nil, pptspec.Design{}, fmt.Errorf("%w: %v", ErrSourceInvalid, err)
	}
	design, err := (DesignLoader{}).Load(project.WorkDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return deck, outline, nil, design, fmt.Errorf("%w: design: %v", ErrRequiredMissing, err)
	}
	if validationErr := pptspec.ValidateDesign(design); err == nil && validationErr != nil {
		return deck, outline, nil, design, fmt.Errorf("%w: %v", ErrSourceInvalid, validationErr)
	}
	return deck, outline, slides, design, nil
}

func (a *ContextAssembler) loadSlideHTML(project model.Project, req ContextRequest, pack *ContextPack, manifest *ContextManifest) {
	ids := []string{}
	for _, loc := range pptspec.FlattenOutline(pack.Outline.Outline) {
		ids = append(ids, loc.Slide.ID)
	}
	if req.Command.Scope.IsSinglePage() {
		ids = append([]string{}, req.Command.Scope.SlideIDs...)
	}
	for _, id := range ids {
		path := filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(id)))
		summary, _, err := (SlideHTMLSummaryLoader{}).Load(path)
		if err != nil {
			manifest.Warnings = append(manifest.Warnings, "slide HTML missing for "+id)
			continue
		}
		pack.SlideHTML.Summaries[id] = summary
		if req.Command.Scope.IsSinglePage() && id == req.Command.Scope.SlideIDs[0] {
			pack.Target.SlideHTMLSummary = &summary
		}
	}
}

func slideSummary(loc pptspec.SlideLocation, s pptspec.SlideSpec, ready bool) SlideSummary {
	summary := SlideSummary{ID: loc.Slide.ID, Ordinal: loc.Ordinal, Section: loc.Section.ID, Title: loc.Slide.Title}
	if loc.Subsection != nil {
		summary.Subsection = loc.Subsection.ID
	}
	if ready {
		summary.Core = s.Core
		summary.Purpose = string(s.Purpose)
		summary.ContentType = string(s.ContentType)
	}
	return summary
}

func componentCandidates(components []model.Component) []ComponentCandidate {
	out := make([]ComponentCandidate, 0, len(components))
	for _, component := range components {
		if component.Disabled || component.ContentState != "ready" {
			continue
		}
		tags := make([]string, len(component.Tags))
		for index, tag := range component.Tags {
			tags[index] = string(tag)
		}
		out = append(out, ComponentCandidate{
			ID: component.ID, Name: component.Name, Description: component.Description,
			Tags: tags,
		})
	}
	return out
}

func skillCandidates(skills []model.RepositorySkill) []SkillCandidate {
	out := make([]SkillCandidate, 0, len(skills))
	for _, skill := range skills {
		if skill.Disabled || skill.ContentState != "ready" {
			continue
		}
		tags := make([]string, len(skill.Tags))
		for index, tag := range skill.Tags {
			tags[index] = string(tag)
		}
		out = append(out, SkillCandidate{
			ID: skill.ID, Name: skill.Name, Description: skill.Description, Tags: tags,
		})
	}
	return out
}

func opaqueID(prefix string, parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%s_%x", prefix, h[:12])
}
