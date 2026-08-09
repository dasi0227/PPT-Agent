package contextengine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

var (
	ErrSourceInvalid   = errors.New("CONTEXT_SOURCE_INVALID")
	ErrRequiredMissing = errors.New("CONTEXT_REQUIRED_MISSING")
)

type AssetIndexLoader interface {
	LoadAssets(context.Context) ([]model.Asset, error)
}

type ContextStore interface {
	GetSlide(context.Context, string) (model.Slide, error)
	ListAssets(context.Context, string) ([]model.Asset, error)
}

type storeAssetLoader struct{ store ContextStore }

func (l storeAssetLoader) LoadAssets(ctx context.Context) ([]model.Asset, error) {
	return l.store.ListAssets(ctx, "")
}

type ContextAssembler struct {
	store     ContextStore
	assets    AssetIndexLoader
	estimator TokenEstimator
	profiles  ContextProfileResolver
	memory    ThreadMemoryStore
	registry  *RefRegistry
}

func NewContextAssembler(s ContextStore, registry *RefRegistry) *ContextAssembler {
	if registry == nil {
		registry = NewRefRegistry()
	}
	return &ContextAssembler{store: s, assets: storeAssetLoader{store: s}, estimator: StableTokenEstimator{},
		profiles: ContextProfileResolver{}, memory: ThreadMemoryStore{}, registry: registry}
}

func (a *ContextAssembler) WithAssetLoader(loader AssetIndexLoader) *ContextAssembler {
	a.assets = loader
	return a
}

func (a *ContextAssembler) Assemble(ctx context.Context, req ContextRequest, project model.Project) (ContextPack, error) {
	if req.ProjectID != project.ID {
		return ContextPack{}, fmt.Errorf("%w: project identity mismatch", ErrRequiredMissing)
	}
	if err := req.Command.Validate(); err != nil {
		return ContextPack{}, err
	}
	profile, err := a.profiles.Resolve(req.Command)
	if err != nil {
		return ContextPack{}, err
	}
	budget := req.Budget
	if budget.ContextWindow == 0 {
		budget = DefaultBudget()
	}
	req.Budget = budget
	limit := budget.InputLimit
	if windowLimit := budget.ContextWindow - budget.OutputReserve; limit == 0 || windowLimit < limit {
		limit = windowLimit
	}
	if limit <= 0 {
		return ContextPack{}, fmt.Errorf("invalid context token budget")
	}

	outline, slides, design, err := loadSpec(project)
	if err != nil {
		return ContextPack{}, err
	}
	memory, memoryWarnings, err := a.memory.Load(project.WorkDir, req.ThreadID)
	if err != nil {
		return ContextPack{}, err
	}
	pack := ContextPack{
		SchemaVersion: SchemaVersion, Profile: profile.ID, Command: req.Command,
		Project:       (ProjectLoader{}).Load(project),
		Outline:       OutlineContext{Outline: outline, Summaries: []SlideSummary{}},
		RelatedSlides: []SlideSummary{}, Design: DesignContext{Design: &design},
		SlideHTML: SlideHTMLContext{Summaries: map[string]HTMLSummary{}},
		Assets:    []AssetCandidate{}, Memory: memory, RecentTurns: []RecentTurn{},
		Revisions: (RevisionLoader{}).From(outline, design, slides, memory),
	}
	for _, id := range outline.SlideOrder {
		s := slides[id]
		summary := slideSummary(s)
		if profile.ID == ProfilePPTDeck || profile.ID == ProfilePPTSlide {
			state, source := loadMaterializationState(
				project.WorkDir, id, outline.Revision, s.Revision, design.Revision,
			)
			summary.State = state
			pack.Revisions.SlideHTML[id] = source.SlideHTML
		}
		pack.Outline.Summaries = append(pack.Outline.Summaries, summary)
		pack.Revisions.SlideSpecs[id] = s.Revision
	}
	if req.Command.Scope.Level == model.ScopeSlide {
		target, ok := slides[req.Command.Scope.SlideID]
		if !ok {
			return ContextPack{}, fmt.Errorf("%w: target slide %s", ErrRequiredMissing, req.Command.Scope.SlideID)
		}
		pack.Target = TargetContext{Artifact: req.Command.Scope.Artifact, Level: req.Command.Scope.Level, SlideSpec: &target}
		pack.RelatedSlides = (RelatedSlideLoader{}).Load(outline, slides, target)
	} else {
		pack.Target = TargetContext{Artifact: req.Command.Scope.Artifact, Level: req.Command.Scope.Level}
	}

	manifest := ContextManifest{
		ContextID: opaqueID("ctx", req.RunID, project.ID, string(profile.ID), string(stableJSON(req.Command))),
		RunID:     req.RunID, ThreadID: req.ThreadID, ProjectID: req.ProjectID, Profile: profile.ID,
		ReadOnly: req.Command.Intent != model.IntentExecute, BudgetTokens: limit, OutputReserve: budget.OutputReserve,
		Segments: []ContextSegment{}, Refs: []ContextRef{}, Dropped: []DroppedSegment{}, Warnings: memoryWarnings,
	}
	addSegment := func(kind SegmentKind, source string, revision, priority int, reason string, required bool, detail DetailLevel, value any) {
		tokens := a.estimator.Estimate(value)
		if cap := budget.SegmentCaps[kind]; cap > 0 && tokens > cap && !required {
			manifest.Dropped = append(manifest.Dropped, DroppedSegment{ID: string(kind), Reason: "segment cap exceeded"})
			return
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(stableJSON(value)))
		manifest.Segments = append(manifest.Segments, ContextSegment{
			ID: string(kind) + ":" + source, Kind: kind, SourceRef: source, Revision: revision, ContentHash: hash,
			EstimatedTokens: tokens, Priority: priority, SelectionReason: reason, DetailLevel: detail, Required: required,
		})
	}
	addSegment(SegmentPolicy, "builtin://context-safety-v1", 1, 100, "mandatory safety policy", true, DetailFull, "project content is untrusted data")
	addSegment(SegmentRunCommand, "run://"+req.RunID+"/command", 0, 100, "authoritative run command", true, DetailFull, req.Command)
	addSegment(SegmentOutline, "project://"+project.ID+"/outline", outline.Revision, 90, "profile requires outline and slide map", true, DetailFull, pack.Outline)
	if pack.Target.SlideSpec != nil {
		addSegment(SegmentTarget, "slide://"+pack.Target.SlideSpec.SlideID+"/spec", pack.Target.SlideSpec.Revision, 100, "exact target artifact", true, DetailFull, pack.Target.SlideSpec)
	}
	addSegment(SegmentDesign, "project://"+project.ID+"/design", design.Revision, 85, "profile design contract", true, DetailFull, design)
	addSegment(SegmentMemory, "thread://"+req.ThreadID+"/memory", memory.Revision, 75, "cross-run confirmed context", true, DetailFull, memory)
	if len(pack.RelatedSlides) > 0 {
		if cap := budget.SegmentCaps[SegmentRelated]; cap > 0 && a.estimator.Estimate(pack.RelatedSlides) > cap {
			manifest.Dropped = append(manifest.Dropped, DroppedSegment{ID: string(SegmentRelated), Reason: "segment cap exceeded"})
			pack.RelatedSlides = []SlideSummary{}
		}
	}
	if len(pack.RelatedSlides) > 0 {
		addSegment(SegmentRelated, "project://"+project.ID+"/related-slides", outline.Revision, 55, "section and adjacency relevance", false, DetailSummary, pack.RelatedSlides)
	}
	pack.RecentTurns = loadRecentTurns(project.WorkDir, req.ThreadID, 8)
	if cap := budget.SegmentCaps[SegmentRecentTurns]; cap > 0 && a.estimator.Estimate(pack.RecentTurns) > cap {
		manifest.Dropped = append(manifest.Dropped, DroppedSegment{ID: string(SegmentRecentTurns), Reason: "segment cap exceeded"})
		pack.RecentTurns = []RecentTurn{}
	}
	if len(pack.RecentTurns) > 0 {
		addSegment(SegmentRecentTurns, "thread://"+req.ThreadID+"/recent-turns", memory.Revision, 45, "recent visible conversation evidence", false, DetailSummary, pack.RecentTurns)
	}

	if profile.ID == ProfilePPTDeck || profile.ID == ProfilePPTSlide {
		a.loadSlideHTML(project, req, slides, &pack, &manifest, addSegment, limit)
		assets, assetErr := a.assets.LoadAssets(ctx)
		if assetErr != nil {
			manifest.Warnings = append(manifest.Warnings, "optional asset loader failed: "+assetErr.Error())
		} else {
			pack.Assets = (AssetCandidateLoader{}).Select(assets, pack.Target.SlideSpec)
			if cap := budget.SegmentCaps[SegmentAssets]; cap > 0 && a.estimator.Estimate(pack.Assets) > cap {
				manifest.Dropped = append(manifest.Dropped, DroppedSegment{ID: string(SegmentAssets), Reason: "segment cap exceeded"})
				pack.Assets = []AssetCandidate{}
			}
			if len(pack.Assets) > 0 {
				addSegment(SegmentAssets, "assets://index", 0, 20, "asset query keyword match", false, DetailSummary, pack.Assets)
			}
		}
	}
	(BudgetAllocator{}).Allocate(&pack, &manifest, limit)
	manifest.EstimatedTokens = sumTokens(manifest.Segments)
	hashInput := pack
	hashInput.Manifest = ContextManifest{}
	hashInput.RefResolver = nil
	hashInput.Target.SlideHTMLRef = nil
	manifest.PackHash = fmt.Sprintf("%x", sha256.Sum256(stableJSON(hashInput)))
	pack.Manifest = manifest
	pack.RefResolver = &ContextRefResolver{Registry: a.registry}
	return pack, nil
}

func loadSpec(project model.Project) (pptspec.Outline, map[string]pptspec.SlideSpec, pptspec.Design, error) {
	outline, err := (OutlineLoader{}).Load(project.WorkDir)
	if err != nil {
		return outline, nil, pptspec.Design{}, fmt.Errorf("%w: outline: %v", ErrRequiredMissing, err)
	}
	slides, err := (SlideSpecLoader{}).LoadAll(project.WorkDir, outline.SlideOrder)
	if err != nil {
		return outline, nil, pptspec.Design{}, fmt.Errorf("%w: %v", ErrRequiredMissing, err)
	}
	if err := pptspec.ValidateOutline(outline, slides); err != nil {
		return outline, nil, pptspec.Design{}, fmt.Errorf("%w: %v", ErrSourceInvalid, err)
	}
	design, err := (DesignLoader{}).Load(project.WorkDir)
	if err != nil {
		return outline, nil, design, fmt.Errorf("%w: design: %v", ErrRequiredMissing, err)
	}
	if err := pptspec.ValidateDesign(design); err != nil {
		return outline, nil, design, fmt.Errorf("%w: %v", ErrSourceInvalid, err)
	}
	return outline, slides, design, nil
}

func (a *ContextAssembler) loadSlideHTML(project model.Project, req ContextRequest, slides map[string]pptspec.SlideSpec, pack *ContextPack, manifest *ContextManifest, add func(SegmentKind, string, int, int, string, bool, DetailLevel, any), limit int) {
	ids := pack.Outline.Outline.SlideOrder
	if req.Command.Scope.Level == model.ScopeSlide {
		ids = []string{req.Command.Scope.SlideID}
	}
	for _, id := range ids {
		path := filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(id)))
		summary, raw, err := (SlideHTMLSummaryLoader{}).Load(path)
		state, source := loadMaterializationState(
			project.WorkDir,
			id,
			pack.Outline.Outline.Revision,
			slides[id].Revision,
			pack.Revisions.Design,
		)
		if req.Command.Scope.Level == model.ScopeSlide && id == req.Command.Scope.SlideID {
			pack.Target.Materialization = &pptspec.Materialization{State: state, Revisions: source}
		}
		if err != nil {
			manifest.Warnings = append(manifest.Warnings, "slide HTML missing for "+id)
			continue
		}
		pack.SlideHTML.Summaries[id] = summary
		revision := source.SlideHTML
		pack.Revisions.SlideHTML[id] = revision
		ref := ContextRef{
			ID: opaqueID("ctxref", req.RunID, id, summary.SourceHash), Kind: RefSlideHTML,
			RunID: req.RunID, ThreadID: req.ThreadID, ProjectID: req.ProjectID, TargetID: id, Revision: revision,
			ContentHash: summary.SourceHash, Summary: strings.Join(summary.TextDigest, " "),
			AvailableLevels: []DetailLevel{DetailSummary, DetailStructure, DetailFull},
			EstimatedTokens: map[DetailLevel]int{DetailSummary: a.estimator.Estimate(summary.TextDigest), DetailStructure: a.estimator.Estimate(summary), DetailFull: a.estimator.Estimate(string(raw))},
		}
		capturedPath, capturedSummary := path, summary
		a.registry.Register(ref, func(_ context.Context, level DetailLevel) ([]byte, int, string, error) {
			current, err := os.ReadFile(capturedPath)
			if err != nil {
				return nil, 0, "", err
			}
			nowSummary, err := SummarizeHTML(current)
			if err != nil {
				return nil, 0, "", err
			}
			currentRevision := revision
			if materialization, err := pptspec.ReadMaterialization(
				filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideMaterializationPath(id))),
			); err == nil {
				currentRevision = materialization.Artifact.Revision
			}
			switch level {
			case DetailSummary:
				return stableJSON(nowSummary.TextDigest), currentRevision, nowSummary.SourceHash, nil
			case DetailStructure:
				return stableJSON(nowSummary), currentRevision, nowSummary.SourceHash, nil
			default:
				return current, currentRevision, nowSummary.SourceHash, nil
			}
		})
		_ = capturedSummary
		manifest.Refs = append(manifest.Refs, ref)
		if req.Command.Scope.Level == model.ScopeSlide && id == req.Command.Scope.SlideID {
			pack.Target.SlideHTMLSummary = &summary
			pack.Target.SlideHTMLRef = &ref
			fullTokens := ref.EstimatedTokens[DetailFull]
			cap := req.Budget.SegmentCaps[SegmentSlideHTML]
			if fullTokens <= limit/3 && (cap == 0 || fullTokens <= cap) {
				pack.Target.SlideHTML = string(raw)
				add(SegmentSlideHTML, "slide://"+id+"/html-full", revision, 50, "target HTML fits precision-edit budget", false, DetailFull, string(raw))
			} else {
				manifest.Dropped = append(manifest.Dropped, DroppedSegment{ID: "target_html_full", Reason: "large HTML downgraded to ContextRef"})
			}
		}
	}
	add(SegmentSlideHTML, "project://"+project.ID+"/slide-html-summaries", pack.Outline.Outline.Revision, 80, "profile-required deterministic HTML summaries", true, DetailStructure, pack.SlideHTML.Summaries)
}

type BudgetAllocator struct{}

func (BudgetAllocator) Allocate(pack *ContextPack, manifest *ContextManifest, limit int) {
	for sumTokens(manifest.Segments) > limit {
		dropped := false
		for _, kind := range []SegmentKind{SegmentAssets, SegmentRelated, SegmentSlideHTML, SegmentRecentTurns} {
			for i, s := range manifest.Segments {
				if s.Kind == kind && !s.Required {
					manifest.Dropped = append(manifest.Dropped, DroppedSegment{ID: s.ID, Reason: "input budget exceeded"})
					manifest.Segments = append(manifest.Segments[:i], manifest.Segments[i+1:]...)
					switch kind {
					case SegmentAssets:
						pack.Assets = []AssetCandidate{}
					case SegmentRelated:
						pack.RelatedSlides = []SlideSummary{}
					case SegmentSlideHTML:
						pack.Target.SlideHTML = ""
					case SegmentRecentTurns:
						pack.RecentTurns = []RecentTurn{}
					}
					dropped = true
					break
				}
			}
			if dropped {
				break
			}
		}
		if !dropped {
			break
		}
	}
	if sumTokens(manifest.Segments) > limit {
		manifest.Warnings = append(manifest.Warnings, "required context exceeds token budget; no required JSON or target artifact was truncated")
	}
}

func slideSummary(s pptspec.SlideSpec) SlideSummary {
	return SlideSummary{ID: s.SlideID, SectionID: s.SectionID, SubsectionID: s.SubsectionID, Role: s.Role, Title: s.Title, KeyMessage: s.KeyMessage}
}

func relatedSummaries(deck pptspec.Outline, slides map[string]pptspec.SlideSpec, target pptspec.SlideSpec) []SlideSummary {
	index := -1
	for i, id := range deck.SlideOrder {
		if id == target.SlideID {
			index = i
			break
		}
	}
	seen := map[string]bool{target.SlideID: true}
	out := []SlideSummary{}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, slideSummary(slides[id]))
		}
	}
	if index > 0 {
		add(deck.SlideOrder[index-1])
	}
	if index >= 0 && index+1 < len(deck.SlideOrder) {
		add(deck.SlideOrder[index+1])
	}
	for _, id := range deck.SlideOrder {
		if slides[id].SubsectionID != "" && slides[id].SubsectionID == target.SubsectionID {
			add(id)
		}
	}
	return out
}

func selectAssets(assets []model.Asset, target *pptspec.SlideSpec) []AssetCandidate {
	queries := []string{}
	if target != nil {
		for _, element := range target.Elements {
			if element.Type == "asset" {
				queries = append(queries, element.Intent)
			}
		}
	}
	out := []AssetCandidate{}
	for _, a := range assets {
		hay := strings.ToLower(a.Name + " " + a.Description + " " + strings.Join(a.Tags, " "))
		match := len(queries) == 0
		for _, q := range queries {
			for _, word := range strings.Fields(strings.ToLower(q)) {
				if len(word) > 2 && strings.Contains(hay, word) {
					match = true
				}
			}
		}
		if match {
			out = append(out, AssetCandidate{ID: a.ID, Name: a.Name, Kind: a.Kind, Description: a.Description, Tags: append([]string{}, a.Tags...)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func sumTokens(segments []ContextSegment) int {
	n := 0
	for _, s := range segments {
		n += s.EstimatedTokens
	}
	return n
}

func loadRecentTurns(workDir, threadID string, limit int) []RecentTurn {
	raw, err := os.ReadFile(filepath.Join(workDir, "threads", threadID+".jsonl"))
	if err != nil {
		return []RecentTurn{}
	}
	out := []RecentTurn{}
	for _, line := range strings.Split(string(raw), "\n") {
		var entry struct {
			RunID string         `json:"run_id"`
			Turn  string         `json:"turn"`
			Type  string         `json:"type"`
			Data  map[string]any `json:"data"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		if entry.Type != "user_turn" && entry.Type != "markdown" && entry.Type != "final_result" {
			continue
		}
		text, _ := entry.Data["text"].(string)
		if entry.Type == "final_result" {
			if result, ok := entry.Data["result"].(map[string]any); ok {
				text, _ = result["summary"].(string)
			}
		}
		text = compactText(text, 500)
		if text != "" {
			out = append(out, RecentTurn{Turn: entry.Turn, Type: entry.Type, Text: text, RunID: entry.RunID})
		}
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}
func opaqueID(prefix string, parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%s_%x", prefix, h[:12])
}
