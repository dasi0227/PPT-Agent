package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var ErrBlueprintRevisionConflict = errors.New("blueprint revision conflict")

type BlueprintService struct {
	store store.Store
	clock func() int64
}

func NewBlueprintService(s store.Store) *BlueprintService {
	return &BlueprintService{store: s, clock: func() int64 { return time.Now().Unix() }}
}

func (s *BlueprintService) EnsureProject(ctx context.Context, projectID string) (blueprint.ProjectView, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return blueprint.ProjectView{}, err
	}
	slides, err := s.store.ListSlides(ctx, projectID)
	if err != nil {
		return blueprint.ProjectView{}, err
	}
	if err := s.migrate(project, slides); err != nil {
		return blueprint.ProjectView{}, err
	}
	return s.read(project, slides)
}

func (s *BlueprintService) GetSlide(ctx context.Context, slideID string) (blueprint.Slide, blueprint.Materialization, error) {
	meta, err := s.store.GetSlide(ctx, slideID)
	if err != nil {
		return blueprint.Slide{}, blueprint.Materialization{}, err
	}
	view, err := s.EnsureProject(ctx, meta.ProjectID)
	if err != nil {
		return blueprint.Slide{}, blueprint.Materialization{}, err
	}
	return view.Slides[slideID], view.States[slideID], nil
}

func (s *BlueprintService) PatchSlide(ctx context.Context, slideID string, expected int, next blueprint.Slide) (blueprint.Slide, error) {
	meta, err := s.store.GetSlide(ctx, slideID)
	if err != nil {
		return blueprint.Slide{}, err
	}
	view, err := s.EnsureProject(ctx, meta.ProjectID)
	if err != nil {
		return blueprint.Slide{}, err
	}
	current := view.Slides[slideID]
	if expected != current.Revision {
		return blueprint.Slide{}, ErrBlueprintRevisionConflict
	}
	next.SchemaVersion, next.SlideID = blueprint.SchemaVersion, slideID
	next.Revision, next.CreatedAt, next.UpdatedAt = current.Revision+1, current.CreatedAt, s.clock()
	if err := blueprint.ValidateSlide(next); err != nil {
		return blueprint.Slide{}, err
	}
	raw, _ := json.MarshalIndent(next, "", "  ")
	project, err := s.store.GetProject(ctx, meta.ProjectID)
	if err != nil {
		return blueprint.Slide{}, err
	}
	path := filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideJSONPath(slideID)))
	if err := atomicWrite(path, raw); err != nil {
		return blueprint.Slide{}, err
	}
	if err := s.store.UpdateSlideMeta(ctx, slideID, next.Title, next.VisualIntent.Archetype); err != nil {
		_ = atomicWrite(path, mustJSON(current))
		return blueprint.Slide{}, err
	}
	if err := s.store.UpdateSlideRevisions(ctx, slideID, next.Revision, meta.PresentationRevision, meta.SourceDeckRevision, meta.SourceBlueprintRevision, meta.SourceDesignRevision); err != nil {
		_ = atomicWrite(path, mustJSON(current))
		return blueprint.Slide{}, err
	}
	return next, nil
}

func (s *BlueprintService) ReplaceDeck(ctx context.Context, projectID string, expected int, next blueprint.Deck) (blueprint.Deck, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return blueprint.Deck{}, err
	}
	view, err := s.EnsureProject(ctx, projectID)
	if err != nil {
		return blueprint.Deck{}, err
	}
	if expected != view.Deck.Revision {
		return blueprint.Deck{}, ErrBlueprintRevisionConflict
	}
	next.SchemaVersion, next.ProjectID = blueprint.SchemaVersion, projectID
	next.Revision, next.CreatedAt, next.UpdatedAt = view.Deck.Revision+1, view.Deck.CreatedAt, s.clock()
	if err := blueprint.ValidateDeck(next, view.Slides); err != nil {
		return blueprint.Deck{}, err
	}
	path := filepath.Join(project.WorkDir, "deck.json")
	if err := atomicWrite(path, mustJSON(next)); err != nil {
		return blueprint.Deck{}, err
	}
	if err := s.store.UpdateProjectRevisions(ctx, projectID, next.Revision, view.DesignSpec.Revision); err != nil {
		_ = atomicWrite(path, mustJSON(view.Deck))
		return blueprint.Deck{}, err
	}
	return next, nil
}

func (s *BlueprintService) ReplaceDesignSpec(ctx context.Context, projectID string, expected int, next blueprint.DesignSpec) (blueprint.DesignSpec, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return blueprint.DesignSpec{}, err
	}
	view, err := s.EnsureProject(ctx, projectID)
	if err != nil {
		return blueprint.DesignSpec{}, err
	}
	if expected != view.DesignSpec.Revision {
		return blueprint.DesignSpec{}, ErrBlueprintRevisionConflict
	}
	next.SchemaVersion = blueprint.SchemaVersion
	next.Revision = view.DesignSpec.Revision + 1
	if err := blueprint.ValidateDesignSpec(next); err != nil {
		return blueprint.DesignSpec{}, err
	}
	path := filepath.Join(project.WorkDir, "design", "design-spec.json")
	if err := atomicWrite(path, mustJSON(next)); err != nil {
		return blueprint.DesignSpec{}, err
	}
	if err := s.store.UpdateProjectRevisions(ctx, projectID, view.Deck.Revision, next.Revision); err != nil {
		_ = atomicWrite(path, mustJSON(view.DesignSpec))
		return blueprint.DesignSpec{}, err
	}
	return next, nil
}

func (s *BlueprintService) migrate(project model.Project, metas []model.Slide) error {
	deckPath := filepath.Join(project.WorkDir, "deck.json")
	if raw, err := os.ReadFile(deckPath); err == nil {
		var deck blueprint.Deck
		if json.Unmarshal(raw, &deck) == nil && deck.SchemaVersion == blueprint.SchemaVersion {
			existing := make(map[string]blueprint.Slide, len(metas))
			complete := true
			for _, meta := range metas {
				var slide blueprint.Slide
				if err := readJSON(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideJSONPath(meta.ID))), &slide); err != nil || slide.SchemaVersion != blueprint.SchemaVersion {
					complete = false
					break
				}
				existing[meta.ID] = slide
			}
			if complete && blueprint.ValidateDeck(deck, existing) == nil {
				return nil
			}
		}
	}
	now := s.clock()
	section := blueprint.Section{ID: "section-main", Number: "01", Title: "正文", Subsections: []blueprint.Subsection{}}
	slides := make(map[string]blueprint.Slide, len(metas))
	order := make([]string, 0, len(metas))
	for _, meta := range metas {
		path := filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideJSONPath(meta.ID)))
		var legacy slidejson.SlideJSON
		raw, readErr := os.ReadFile(path)
		if readErr == nil {
			var already blueprint.Slide
			if json.Unmarshal(raw, &already) == nil && already.SchemaVersion == blueprint.SchemaVersion {
				slides[meta.ID] = already
				order = append(order, meta.ID)
				continue
			}
			_ = atomicWrite(filepath.Join(filepath.Dir(path), "slide.legacy.json"), raw)
			_ = json.Unmarshal(raw, &legacy)
		}
		title := firstNonEmpty(legacy.Title, meta.Title, fmt.Sprintf("第 %d 页", meta.Idx+1))
		summary := firstNonEmpty(legacy.ContentIntent, legacy.Subtitle, title)
		description := "使用清晰的信息层级表达本页核心信息"
		if legacy.ChartIntent != nil {
			description = strings.TrimSpace(legacy.ChartIntent.Type + " " + legacy.ChartIntent.DataHint)
		}
		role := "context"
		if meta.Idx == 0 {
			role = "cover"
		}
		slide := blueprint.Slide{
			SchemaVersion: blueprint.SchemaVersion, Revision: 1, SlideID: meta.ID,
			SectionID: section.ID, Role: role, Title: title, KeyMessage: summary,
			Content:      blueprint.Content{Summary: summary, Points: nonNil(legacy.Bullets)},
			VisualIntent: blueprint.VisualIntent{Archetype: firstNonEmpty(legacy.Layout, meta.Layout, "content"), Description: description, AssetQueries: []string{}},
			SpeakerNotes: legacy.Notes, CreatedAt: now, UpdatedAt: now,
		}
		if err := atomicWrite(path, mustJSON(slide)); err != nil {
			return err
		}
		slides[meta.ID] = slide
		order = append(order, meta.ID)
		if err := s.store.UpdateSlideRevisions(context.Background(), meta.ID, 1, meta.PresentationRevision, meta.SourceDeckRevision, meta.SourceBlueprintRevision, meta.SourceDesignRevision); err != nil {
			return err
		}
	}
	deck := blueprint.Deck{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1, ProjectID: project.ID, Title: project.Title,
		Goal: project.Title, Audience: "待明确", Language: "zh-CN", CoreThesis: project.Title,
		NarrativeArc: "背景 → 核心内容 → 结论", Sections: []blueprint.Section{section},
		SlideOrder: order, CreatedAt: now, UpdatedAt: now,
	}
	if err := blueprint.ValidateDeck(deck, slides); err != nil {
		return err
	}
	if err := atomicWrite(deckPath, mustJSON(deck)); err != nil {
		return err
	}
	designPath := filepath.Join(project.WorkDir, "design", "design-spec.json")
	if raw, err := os.ReadFile(designPath); err == nil {
		_ = atomicWrite(filepath.Join(project.WorkDir, "design", "design-spec.legacy.json"), raw)
	}
	design := defaultDesign()
	if err := atomicWrite(designPath, mustJSON(design)); err != nil {
		return err
	}
	return s.store.UpdateProjectRevisions(context.Background(), project.ID, 1, 1)
}

func (s *BlueprintService) read(project model.Project, metas []model.Slide) (blueprint.ProjectView, error) {
	var deck blueprint.Deck
	if err := readJSON(filepath.Join(project.WorkDir, "deck.json"), &deck); err != nil {
		return blueprint.ProjectView{}, err
	}
	var design blueprint.DesignSpec
	if err := readJSON(filepath.Join(project.WorkDir, "design", "design-spec.json"), &design); err != nil {
		return blueprint.ProjectView{}, err
	}
	out := blueprint.ProjectView{Deck: deck, DesignSpec: design, Slides: map[string]blueprint.Slide{}, States: map[string]blueprint.Materialization{}}
	for _, meta := range metas {
		var slide blueprint.Slide
		if err := readJSON(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideJSONPath(meta.ID))), &slide); err != nil {
			return blueprint.ProjectView{}, err
		}
		out.Slides[meta.ID] = slide
		_, statErr := os.Stat(filepath.Join(project.WorkDir, filepath.FromSlash(meta.HTMLPath)))
		revs := model.MaterializationRevisions{
			Presentation: meta.PresentationRevision, Deck: meta.SourceDeckRevision,
			SlideBlueprint: meta.SourceBlueprintRevision, Design: meta.SourceDesignRevision,
		}
		out.States[meta.ID] = blueprint.Materialization{
			State:     string(model.DeriveMaterializationState(statErr == nil, deck.Revision, slide.Revision, design.Revision, revs)),
			Revisions: revs,
		}
	}
	if err := blueprint.ValidateDeck(deck, out.Slides); err != nil {
		return blueprint.ProjectView{}, err
	}
	return out, nil
}

func defaultDesign() blueprint.DesignSpec {
	return blueprint.DesignSpec{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1,
		Canvas:     map[string]any{"width": 1280, "height": 720, "ratio": "16:9"},
		Palette:    []string{"#111827", "#ffffff", "#2563eb"},
		Typography: map[string]any{"heading": "Inter, sans-serif", "body": "Inter, sans-serif"},
		Spacing:    map[string]any{"unit": 8}, Radius: map[string]any{"card": 16},
		Shadows:      map[string]any{"card": "0 12px 36px rgba(15,23,42,.12)"},
		LayoutSystem: map[string]any{"grid": "12-col", "rhythm": "generous", "density": "medium"},
		Signature:    "minimal geometric accent", Motion: map[string]any{"policy": "restrained"},
	}
}

func atomicWrite(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".artifact-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func readJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func mustJSON(v any) []byte {
	raw, _ := json.MarshalIndent(v, "", "  ")
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
