package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

func DefaultToolRegistry(pack contextengine.ContextPack) *ToolRegistry {
	registry := NewToolRegistry()
	register := func(desc ToolDescriptor) {
		if err := registry.Register(desc); err != nil {
			panic(err)
		}
	}
	register(ToolDescriptor{
		Name: "read_context_ref", Capabilities: []Capability{CapabilityReadContext}, Risk: RiskRead,
		Stages: []Stage{StageExecute, StageRepair},
		Strategies: []ExecutionStrategy{
			StrategyRespond, StrategyDirectAction, StrategyCompactWorkflow, StrategyFullPEV,
		},
		Tool: &readContextRefTool{pack: pack, remaining: pack.Manifest.BudgetTokens - pack.Manifest.EstimatedTokens},
	})
	register(ToolDescriptor{
		Name: "read_staged_artifact", Capabilities: []Capability{CapabilityReadBlueprint}, Risk: RiskRead,
		Stages: []Stage{StageExecute, StageRepair},
		Strategies: []ExecutionStrategy{
			StrategyDirectAction, StrategyCompactWorkflow, StrategyFullPEV,
		},
		Tool: readStagedArtifactTool{},
	})
	register(ToolDescriptor{
		Name: "write_staged_deck_blueprint", Capabilities: []Capability{CapabilityWriteBlueprint}, Risk: RiskWrite,
		Stages: []Stage{StageExecute, StageRepair},
		Strategies: []ExecutionStrategy{
			StrategyCompactWorkflow, StrategyFullPEV,
		},
		Mutates: []ArtifactKind{ArtifactDeck},
		Tool:    writeDeckBlueprintTool{pack: pack},
	})
	register(ToolDescriptor{
		Name: "write_staged_slide_blueprint", Capabilities: []Capability{CapabilityWriteBlueprint}, Risk: RiskWrite,
		Stages:     []Stage{StageExecute, StageRepair},
		Strategies: []ExecutionStrategy{StrategyCompactWorkflow, StrategyFullPEV},
		Mutates:    []ArtifactKind{ArtifactSlide},
		Tool:       writeSlideBlueprintTool{pack: pack},
	})
	register(ToolDescriptor{
		Name: "patch_staged_slide_blueprint", Capabilities: []Capability{CapabilityWriteBlueprint}, Risk: RiskWrite,
		Stages: []Stage{StageExecute}, Strategies: []ExecutionStrategy{StrategyDirectAction},
		Mutates: []ArtifactKind{ArtifactSlide}, Tool: patchSlideBlueprintTool{},
	})
	register(ToolDescriptor{
		Name: "write_staged_design_spec", Capabilities: []Capability{CapabilityWriteDesignSpec}, Risk: RiskWrite,
		Stages: []Stage{StageExecute, StageRepair},
		Strategies: []ExecutionStrategy{
			StrategyCompactWorkflow, StrategyFullPEV,
		},
		Mutates: []ArtifactKind{ArtifactDesign},
		Tool:    writeDesignSpecTool{pack: pack},
	})
	register(ToolDescriptor{
		Name: "write_staged_presentation", Capabilities: []Capability{CapabilityWritePresentation}, Risk: RiskWrite,
		Stages:     []Stage{StageExecute, StageRepair},
		Strategies: []ExecutionStrategy{StrategyCompactWorkflow, StrategyFullPEV},
		Mutates:    []ArtifactKind{ArtifactPresentation},
		Tool:       writePresentationTool{},
	})
	register(ToolDescriptor{
		Name: "patch_staged_presentation", Capabilities: []Capability{CapabilityWritePresentation}, Risk: RiskWrite,
		Stages: []Stage{StageExecute}, Strategies: []ExecutionStrategy{StrategyDirectAction},
		Mutates: []ArtifactKind{ArtifactPresentation}, Tool: patchPresentationTool{},
	})
	register(ToolDescriptor{
		Name: "search_assets", Capabilities: []Capability{CapabilitySearchAssets}, Risk: RiskRead,
		Stages: []Stage{StageExecute, StageRepair},
		Strategies: []ExecutionStrategy{
			StrategyCompactWorkflow, StrategyFullPEV,
		},
		Tool: searchAssetsTool{pack: pack},
	})
	register(ToolDescriptor{
		Name: "finish_step", Capabilities: []Capability{CapabilityControl}, Risk: RiskControl,
		Stages: []Stage{StageExecute, StageRepair},
		Strategies: []ExecutionStrategy{
			StrategyDirectAction, StrategyCompactWorkflow, StrategyFullPEV,
		},
		Tool: finishStepTool{},
	})
	return registry
}

type readContextRefTool struct {
	pack      contextengine.ContextPack
	mu        sync.Mutex
	remaining int
}

func (*readContextRefTool) Name() string { return "read_context_ref" }
func (*readContextRefTool) Description() string {
	return "Read an opaque reference from this run's ContextManifest."
}
func (*readContextRefTool) Parameters() map[string]any {
	return objectSchema([]string{"ref_id", "detail"}, map[string]any{
		"ref_id": map[string]any{"type": "string"},
		"detail": map[string]any{"type": "string", "enum": []string{"summary", "structure", "full"}},
	})
}
func (t *readContextRefTool) Execute(ctx context.Context, input ToolInput) ToolResult {
	if t.pack.RefResolver == nil {
		return toolFailure("CONTEXT_REF_NOT_FOUND", "context ref resolver is unavailable", false)
	}
	refID, _ := input.Args["ref_id"].(string)
	detail, _ := input.Args["detail"].(string)
	t.mu.Lock()
	defer t.mu.Unlock()
	result, err := t.pack.RefResolver.Read(ctx, contextengine.RefReadRequest{
		RunID: t.pack.Manifest.RunID, ThreadID: t.pack.Manifest.ThreadID, ProjectID: t.pack.Manifest.ProjectID,
		RefID: refID, Detail: contextengine.DetailLevel(detail), RemainingBudget: t.remaining,
	})
	if err != nil {
		return toolFailure("CONTEXT_REF_READ_FAILED", err.Error(), false)
	}
	t.remaining -= result.EstimatedTokens
	return SuccessfulToolResult(result.Content)
}

type readStagedArtifactTool struct{}

func (readStagedArtifactTool) Name() string { return "read_staged_artifact" }
func (readStagedArtifactTool) Description() string {
	return "Read a declared artifact from staging, falling back to the committed copy."
}
func (readStagedArtifactTool) Parameters() map[string]any {
	return objectSchema([]string{"artifact_id"}, map[string]any{"artifact_id": map[string]any{"type": "string"}})
}
func (readStagedArtifactTool) Execute(_ context.Context, input ToolInput) ToolResult {
	ref, ok := targetByID(input.Step, stringArg(input.Args, "artifact_id"))
	if !ok || input.Transaction == nil {
		return toolFailure("CAPABILITY_DENIED", "artifact is not declared by this step", false)
	}
	raw, err := input.Transaction.Read(ref)
	if err != nil {
		return toolFailure("ARTIFACT_READ_FAILED", err.Error(), true)
	}
	return SuccessfulToolResult(string(raw))
}

type writeDeckBlueprintTool struct{ pack contextengine.ContextPack }

func (writeDeckBlueprintTool) Name() string { return "write_staged_deck_blueprint" }
func (writeDeckBlueprintTool) Description() string {
	return "Stage the complete canonical deck blueprint. New decks may include complete slide blueprints in slides."
}
func (writeDeckBlueprintTool) Parameters() map[string]any {
	return objectSchema([]string{"deck"}, map[string]any{
		"deck":   map[string]any{"type": "object"},
		"slides": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
	})
}
func (t writeDeckBlueprintTool) Execute(_ context.Context, input ToolInput) ToolResult {
	if input.Transaction == nil {
		return toolFailure("STAGING_REQUIRED", "staging transaction is required", false)
	}
	if _, ok := targetByKindID(input.Step, ArtifactDeck, t.pack.Project.ID); !ok {
		return toolFailure("CAPABILITY_DENIED", "deck is not declared by this step", false)
	}
	deckRaw, err := json.Marshal(input.Args["deck"])
	if err != nil {
		return toolFailure("BLUEPRINT_INVALID", err.Error(), true)
	}
	var deck blueprint.Deck
	if err := json.Unmarshal(deckRaw, &deck); err != nil {
		return toolFailure("BLUEPRINT_INVALID", err.Error(), true)
	}
	now := time.Now().Unix()
	deck.SchemaVersion, deck.ProjectID = blueprint.SchemaVersion, t.pack.Project.ID
	deck.Revision = t.pack.Deck.Deck.Revision + 1
	deck.CreatedAt = t.pack.Deck.Deck.CreatedAt
	if deck.CreatedAt == 0 {
		deck.CreatedAt = now
	}
	deck.UpdatedAt = now
	rawSlides, _ := json.Marshal(input.Args["slides"])
	var slides []blueprint.Slide
	_ = json.Unmarshal(rawSlides, &slides)
	byID := map[string]blueprint.Slide{}
	for _, slide := range slides {
		if slide.SlideID == "" {
			return toolFailure("BLUEPRINT_INVALID", "supplied slide_id is required", true)
		}
		if _, ok := targetByKindID(input.Step, ArtifactSlide, slide.SlideID); !ok {
			return toolFailure("CAPABILITY_DENIED", "supplied slide is outside the declared step scope", false)
		}
		normalizeSlide(&slide, t.pack, now)
		byID[slide.SlideID] = slide
	}
	for _, id := range deck.SlideOrder {
		if _, ok := byID[id]; ok {
			continue
		}
		ref := blueprintSlideRef(id)
		raw, readErr := input.Transaction.Read(ref)
		if readErr != nil {
			return toolFailure("BLUEPRINT_REFERENCE_BROKEN", "missing slide blueprint "+id, true)
		}
		var slide blueprint.Slide
		if json.Unmarshal(raw, &slide) != nil {
			return toolFailure("BLUEPRINT_INVALID", "invalid existing slide "+id, true)
		}
		byID[id] = slide
	}
	if err := blueprint.ValidateDeck(deck, byID); err != nil {
		return toolFailure("BLUEPRINT_INVALID", err.Error(), true)
	}
	artifacts := []ArtifactRef{}
	deckRef := deckRef(t.pack)
	if _, err := input.Transaction.Stage(deckRef, input.Step.ID, prettyJSON(deck)); err != nil {
		return toolFailure("STAGING_FAILED", err.Error(), true)
	}
	artifacts = append(artifacts, deckRef)
	for _, id := range deck.SlideOrder {
		if _, supplied := findSlide(slides, id); !supplied {
			continue
		}
		ref := blueprintSlideRef(id)
		if _, err := input.Transaction.Stage(ref, input.Step.ID, prettyJSON(byID[id])); err != nil {
			return toolFailure("STAGING_FAILED", err.Error(), true)
		}
		artifacts = append(artifacts, ref)
	}
	return SuccessfulToolResult("deck blueprint staged", artifacts...)
}

type writeSlideBlueprintTool struct{ pack contextengine.ContextPack }

func (writeSlideBlueprintTool) Name() string { return "write_staged_slide_blueprint" }
func (writeSlideBlueprintTool) Description() string {
	return "Stage one canonical slide blueprint. The slide_id must be a declared target."
}
func (writeSlideBlueprintTool) Parameters() map[string]any {
	return objectSchema([]string{"slide_id", "slide"}, map[string]any{
		"slide_id": map[string]any{"type": "string"}, "slide": map[string]any{"type": "object"},
	})
}
func (t writeSlideBlueprintTool) Execute(_ context.Context, input ToolInput) ToolResult {
	id := stringArg(input.Args, "slide_id")
	ref, ok := targetByKindID(input.Step, ArtifactSlide, id)
	if !ok || input.Transaction == nil {
		return toolFailure("CAPABILITY_DENIED", "slide is not declared by this step", false)
	}
	raw, err := json.Marshal(input.Args["slide"])
	if err != nil {
		return toolFailure("BLUEPRINT_INVALID", err.Error(), true)
	}
	var slide blueprint.Slide
	if err := json.Unmarshal(raw, &slide); err != nil {
		return toolFailure("BLUEPRINT_INVALID", err.Error(), true)
	}
	slide.SlideID = id
	normalizeSlide(&slide, t.pack, time.Now().Unix())
	if err := blueprint.ValidateSlide(slide); err != nil {
		return toolFailure("BLUEPRINT_INVALID", err.Error(), true)
	}
	if _, err := input.Transaction.Stage(ref, input.Step.ID, prettyJSON(slide)); err != nil {
		return toolFailure("STAGING_FAILED", err.Error(), true)
	}
	return SuccessfulToolResult("slide blueprint staged", ref)
}

type patchSlideBlueprintTool struct{}

func (patchSlideBlueprintTool) Name() string { return "patch_staged_slide_blueprint" }
func (patchSlideBlueprintTool) Description() string {
	return "Patch exactly one scalar field on the declared slide blueprint."
}
func (patchSlideBlueprintTool) Parameters() map[string]any {
	return objectSchema([]string{"slide_id", "field", "value"}, map[string]any{
		"slide_id": map[string]any{"type": "string"},
		"field":    map[string]any{"type": "string", "enum": []string{"title", "key_message", "summary"}},
		"value":    map[string]any{"type": "string"},
	})
}
func (patchSlideBlueprintTool) Execute(_ context.Context, input ToolInput) ToolResult {
	id := stringArg(input.Args, "slide_id")
	ref, ok := targetByKindID(input.Step, ArtifactSlide, id)
	if !ok || input.Transaction == nil {
		return toolFailure("CAPABILITY_DENIED", "slide is not declared by this step", false)
	}
	raw, err := input.Transaction.Read(ref)
	if err != nil {
		return toolFailure("ARTIFACT_READ_FAILED", err.Error(), false)
	}
	var slide blueprint.Slide
	if err := json.Unmarshal(raw, &slide); err != nil {
		return toolFailure("BLUEPRINT_INVALID", err.Error(), false)
	}
	value := stringArg(input.Args, "value")
	if value == "" {
		return toolFailure("BLUEPRINT_INVALID", "patch value is required", true)
	}
	switch stringArg(input.Args, "field") {
	case "title":
		slide.Title = value
	case "key_message":
		slide.KeyMessage = value
	case "summary":
		slide.Content.Summary = value
	default:
		return toolFailure(ErrDirectActionUpgrade.Error(), "requested field is not a direct-action scalar", false)
	}
	slide.Revision++
	slide.UpdatedAt = time.Now().Unix()
	if err := blueprint.ValidateSlide(slide); err != nil {
		return toolFailure("BLUEPRINT_INVALID", err.Error(), true)
	}
	if _, err := input.Transaction.Stage(ref, input.Step.ID, prettyJSON(slide)); err != nil {
		return toolFailure("STAGING_FAILED", err.Error(), true)
	}
	return SuccessfulToolResult("slide blueprint field patched", ref)
}

type writeDesignSpecTool struct{ pack contextengine.ContextPack }

func (writeDesignSpecTool) Name() string { return "write_staged_design_spec" }
func (writeDesignSpecTool) Description() string {
	return "Stage the canonical design specification for a deck-level design change."
}
func (writeDesignSpecTool) Parameters() map[string]any {
	return objectSchema([]string{"design"}, map[string]any{"design": map[string]any{"type": "object"}})
}
func (t writeDesignSpecTool) Execute(_ context.Context, input ToolInput) ToolResult {
	ref, ok := targetByKindID(input.Step, ArtifactDesign, t.pack.Project.ID)
	if !ok || input.Transaction == nil {
		return toolFailure("CAPABILITY_DENIED", "design spec is not declared by this step", false)
	}
	raw, err := json.Marshal(input.Args["design"])
	if err != nil {
		return toolFailure("DESIGN_INVALID", err.Error(), true)
	}
	var design blueprint.DesignSpec
	if err := json.Unmarshal(raw, &design); err != nil {
		return toolFailure("DESIGN_INVALID", err.Error(), true)
	}
	design.SchemaVersion = blueprint.SchemaVersion
	design.Revision = t.pack.Revisions.Design + 1
	if err := blueprint.ValidateDesignSpec(design); err != nil {
		return toolFailure("DESIGN_INVALID", err.Error(), true)
	}
	if _, err := input.Transaction.Stage(ref, input.Step.ID, prettyJSON(design)); err != nil {
		return toolFailure("STAGING_FAILED", err.Error(), true)
	}
	return SuccessfulToolResult("design spec staged", ref)
}

type writePresentationTool struct{}

func (writePresentationTool) Name() string { return "write_staged_presentation" }
func (writePresentationTool) Description() string {
	return "Stage complete HTML for one declared presentation slide."
}
func (writePresentationTool) Parameters() map[string]any {
	return objectSchema([]string{"slide_id", "html"}, map[string]any{
		"slide_id": map[string]any{"type": "string"}, "html": map[string]any{"type": "string"},
	})
}
func (writePresentationTool) Execute(_ context.Context, input ToolInput) ToolResult {
	id := stringArg(input.Args, "slide_id")
	ref, ok := targetByKindID(input.Step, ArtifactPresentation, id)
	if !ok || input.Transaction == nil {
		return toolFailure("CAPABILITY_DENIED", "presentation slide is not declared by this step", false)
	}
	html, _ := input.Args["html"].(string)
	if strings.TrimSpace(html) == "" {
		return toolFailure("HTML_INVALID", "html is required", true)
	}
	if _, err := input.Transaction.Stage(ref, input.Step.ID, []byte(html)); err != nil {
		return toolFailure("STAGING_FAILED", err.Error(), true)
	}
	return SuccessfulToolResult("presentation slide staged", ref)
}

type patchPresentationTool struct{}

func (patchPresentationTool) Name() string { return "patch_staged_presentation" }
func (patchPresentationTool) Description() string {
	return "Replace one exact, uniquely matched text value in a declared presentation HTML artifact."
}
func (patchPresentationTool) Parameters() map[string]any {
	return objectSchema([]string{"slide_id", "old_text", "new_text"}, map[string]any{
		"slide_id": map[string]any{"type": "string"},
		"old_text": map[string]any{"type": "string"},
		"new_text": map[string]any{"type": "string"},
	})
}
func (patchPresentationTool) Execute(_ context.Context, input ToolInput) ToolResult {
	id := stringArg(input.Args, "slide_id")
	ref, ok := targetByKindID(input.Step, ArtifactPresentation, id)
	if !ok || input.Transaction == nil {
		return toolFailure("CAPABILITY_DENIED", "presentation slide is not declared by this step", false)
	}
	oldText := stringArg(input.Args, "old_text")
	newText := stringArg(input.Args, "new_text")
	if oldText == "" || newText == "" {
		return toolFailure("HTML_PATCH_INVALID", "old_text and new_text are required", true)
	}
	raw, err := input.Transaction.Read(ref)
	if err != nil {
		return toolFailure("ARTIFACT_READ_FAILED", err.Error(), false)
	}
	if strings.Count(string(raw), oldText) != 1 {
		return toolFailure(ErrDirectActionUpgrade.Error(), "patch anchor must match exactly once", false)
	}
	updated := strings.Replace(string(raw), oldText, newText, 1)
	if _, err := input.Transaction.Stage(ref, input.Step.ID, []byte(updated)); err != nil {
		return toolFailure("STAGING_FAILED", err.Error(), true)
	}
	return SuccessfulToolResult("presentation text patched", ref)
}

type searchAssetsTool struct{ pack contextengine.ContextPack }

func (searchAssetsTool) Name() string { return "search_assets" }
func (searchAssetsTool) Description() string {
	return "Search the asset candidates already selected into the ContextPack."
}
func (searchAssetsTool) Parameters() map[string]any {
	return objectSchema([]string{"query"}, map[string]any{"query": map[string]any{"type": "string"}})
}
func (t searchAssetsTool) Execute(_ context.Context, input ToolInput) ToolResult {
	query := strings.ToLower(stringArg(input.Args, "query"))
	matches := []contextengine.AssetCandidate{}
	for _, asset := range t.pack.Assets {
		haystack := strings.ToLower(asset.Name + " " + asset.Description + " " + strings.Join(asset.Tags, " "))
		if query == "" || strings.Contains(haystack, query) {
			matches = append(matches, asset)
		}
	}
	raw, _ := json.Marshal(matches)
	return SuccessfulToolResult(string(raw))
}

type finishStepTool struct{}

func (finishStepTool) Name() string { return "finish_step" }
func (finishStepTool) Description() string {
	return "Finish the current workflow step after required staged writes are complete."
}
func (finishStepTool) Parameters() map[string]any {
	return objectSchema([]string{"summary"}, map[string]any{"summary": map[string]any{"type": "string"}})
}
func (finishStepTool) Execute(_ context.Context, input ToolInput) ToolResult {
	return SuccessfulToolResult(stringArg(input.Args, "summary"))
}

func deterministicStep(input StepInput) (StepResult, error) {
	switch input.Step.Kind {
	case StepReplaceDeck:
		return deterministicDeck(input)
	case StepPatchBlueprint:
		return deterministicBlueprint(input)
	case StepApplyDesign:
		return StepResult{Summary: "Existing DesignSpec confirmed", Artifacts: []ArtifactRef{}, Issues: []Issue{}}, nil
	case StepMaterializeSlide, StepReviseSlide:
		return deterministicPresentation(input)
	case StepRepair:
		return deterministicRepair(input)
	default:
		return StepResult{Summary: "Step completed", Artifacts: []ArtifactRef{}, Issues: []Issue{}}, nil
	}
}

func deterministicDeck(input StepInput) (StepResult, error) {
	pack := input.Context
	deck := pack.Deck.Deck
	now := time.Now().Unix()
	deck.Revision++
	deck.UpdatedAt = now
	deck.Goal = firstValue(input.Step.Instruction, deck.Goal)
	if len(deck.Sections) == 0 {
		deck.Sections = []blueprint.Section{{ID: "section-main", Number: "01", Title: "核心内容", Subsections: []blueprint.Subsection{}}}
	}
	artifacts := []ArtifactRef{}
	if len(deck.SlideOrder) == 0 {
		slideTargets := targetsOfKind(input.Step, ArtifactSlide)
		if len(slideTargets) == 0 {
			return StepResult{}, fmt.Errorf("new deck plan has no declared slide targets")
		}
		for index, target := range slideTargets {
			id := target.ID
			deck.SlideOrder = append(deck.SlideOrder, id)
			role := "context"
			if index == 0 {
				role = "cover"
			} else if index == len(slideTargets)-1 {
				role = "closing"
			}
			slide := blueprint.Slide{
				SchemaVersion: blueprint.SchemaVersion, Revision: 1, SlideID: id,
				SectionID: "section-main", Role: role,
				Title:      fmt.Sprintf("%s · %d", pack.Project.Title, index+1),
				KeyMessage: firstValue(pack.WorkSpec.Instruction, pack.Project.Title),
				Content:    blueprint.Content{Summary: firstValue(pack.WorkSpec.Instruction, pack.Project.Title), Points: []string{}},
				VisualIntent: blueprint.VisualIntent{
					Archetype: "content", Description: "清晰的 16:9 信息层级布局", AssetQueries: []string{},
				},
				CreatedAt: now, UpdatedAt: now,
			}
			ref := blueprintSlideRef(id)
			if _, err := input.Transaction.Stage(ref, input.Step.ID, prettyJSON(slide)); err != nil {
				return StepResult{}, err
			}
			artifacts = append(artifacts, ref)
		}
	}
	ref := deckRef(pack)
	if _, err := input.Transaction.Stage(ref, input.Step.ID, prettyJSON(deck)); err != nil {
		return StepResult{}, err
	}
	artifacts = append([]ArtifactRef{ref}, artifacts...)
	return StepResult{Summary: "Deck blueprint staged", Artifacts: artifacts, Issues: []Issue{}}, nil
}

func deterministicBlueprint(input StepInput) (StepResult, error) {
	artifacts := []ArtifactRef{}
	for _, ref := range input.Step.Targets {
		if ref.Kind != ArtifactSlide {
			continue
		}
		raw, err := input.Transaction.Read(ref)
		if err != nil {
			continue
		}
		var slide blueprint.Slide
		if json.Unmarshal(raw, &slide) != nil {
			continue
		}
		slide.Revision++
		slide.UpdatedAt = time.Now().Unix()
		applyLocalBlueprintInstruction(&slide, input.Step.Instruction)
		if _, err := input.Transaction.Stage(ref, input.Step.ID, prettyJSON(slide)); err != nil {
			return StepResult{}, err
		}
		artifacts = append(artifacts, ref)
	}
	return StepResult{Summary: "Slide blueprints staged", Artifacts: artifacts, Issues: []Issue{}}, nil
}

func applyLocalBlueprintInstruction(slide *blueprint.Slide, instruction string) {
	value := strings.TrimSpace(instruction)
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "title") || strings.Contains(value, "标题"):
		slide.Title = extractPatchValue(value, slide.Title)
	case strings.Contains(lower, "summary") || strings.Contains(value, "摘要"):
		slide.Content.Summary = extractPatchValue(value, slide.Content.Summary)
	default:
		slide.KeyMessage = extractPatchValue(value, slide.KeyMessage)
	}
}

func extractPatchValue(instruction, fallback string) string {
	for _, separator := range []string{"：", ":", "为", "成"} {
		if index := strings.LastIndex(instruction, separator); index >= 0 {
			if value := strings.Trim(strings.TrimSpace(instruction[index+len(separator):]), `"'“”`); value != "" {
				return value
			}
		}
	}
	return firstValue(instruction, fallback)
}

func deterministicPresentation(input StepInput) (StepResult, error) {
	artifacts := []ArtifactRef{}
	for _, ref := range input.Step.Targets {
		if ref.Kind != ArtifactPresentation {
			continue
		}
		slide := blueprintFor(input.Context, ref.ID)
		if input.State.Strategy == StrategyDirectAction {
			if slide == nil {
				return StepResult{}, ErrDirectActionUpgrade
			}
			raw, err := input.Transaction.Read(ref)
			if err != nil {
				return StepResult{}, ErrDirectActionUpgrade
			}
			anchor := slide.KeyMessage
			lower := strings.ToLower(input.Step.Instruction)
			if strings.Contains(lower, "title") || strings.Contains(input.Step.Instruction, "标题") {
				anchor = slide.Title
			}
			if anchor == "" || strings.Count(string(raw), htmlEscape(anchor)) != 1 {
				return StepResult{
					Issues: []Issue{{
						Code: ErrDirectActionUpgrade.Error(), Severity: SeverityInfo,
						Artifact: ref, Evidence: "local patch anchor is missing or ambiguous", Verifier: "runtime",
					}},
				}, ErrDirectActionUpgrade
			}
			value := htmlEscape(extractPatchValue(input.Step.Instruction, anchor))
			html := strings.Replace(string(raw), htmlEscape(anchor), value, 1)
			if _, err := input.Transaction.Stage(ref, input.Step.ID, []byte(html)); err != nil {
				return StepResult{}, err
			}
			artifacts = append(artifacts, ref)
			continue
		}
		title, message := ref.ID, input.Step.Instruction
		if slide != nil {
			title, message = slide.Title, slide.KeyMessage
		}
		html := defaultSlideHTML(title, message)
		if _, err := input.Transaction.Stage(ref, input.Step.ID, []byte(html)); err != nil {
			return StepResult{}, err
		}
		artifacts = append(artifacts, ref)
	}
	return StepResult{Summary: "Presentation HTML staged", Artifacts: artifacts, Issues: []Issue{}}, nil
}

func deterministicRepair(input StepInput) (StepResult, error) {
	presentations := targetsOfKind(input.Step, ArtifactPresentation)
	if len(presentations) > 0 {
		repair := input
		repair.Step.Kind = StepMaterializeSlide
		repair.Step.Targets = presentations
		return deterministicPresentation(repair)
	}
	slides := targetsOfKind(input.Step, ArtifactSlide)
	if len(slides) > 0 {
		repair := input
		repair.Step.Kind = StepPatchBlueprint
		repair.Step.Targets = slides
		return deterministicBlueprint(repair)
	}
	if len(targetsOfKind(input.Step, ArtifactDeck)) > 0 {
		repair := input
		repair.Step.Kind = StepReplaceDeck
		return deterministicDeck(repair)
	}
	return StepResult{Summary: "No deterministic repair target", Artifacts: []ArtifactRef{}, Issues: []Issue{}}, nil
}

func defaultSlideHTML(title, message string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<link rel="stylesheet" href="../../common/tokens.css"><link rel="stylesheet" href="../../common/base.css">` +
		`</head><body><main class="slide-stage"><section><h1>` + htmlEscape(title) + `</h1><p>` + htmlEscape(message) +
		`</p></section></main></body></html>`
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return replacer.Replace(value)
}

func normalizeSlide(slide *blueprint.Slide, pack contextengine.ContextPack, now int64) {
	slide.SchemaVersion = blueprint.SchemaVersion
	if current := pack.Revisions.Slides[slide.SlideID]; current > 0 {
		slide.Revision = current + 1
	} else if slide.Revision < 1 {
		slide.Revision = 1
	}
	if slide.CreatedAt == 0 {
		slide.CreatedAt = now
	}
	slide.UpdatedAt = now
	if slide.Content.Points == nil {
		slide.Content.Points = []string{}
	}
	if slide.VisualIntent.AssetQueries == nil {
		slide.VisualIntent.AssetQueries = []string{}
	}
}

func targetByID(step WorkflowStep, id string) (ArtifactRef, bool) {
	for _, target := range step.Targets {
		if target.ID == id {
			return target, true
		}
	}
	return ArtifactRef{}, false
}

func targetByKindID(step WorkflowStep, kind ArtifactKind, id string) (ArtifactRef, bool) {
	for _, target := range step.Targets {
		if target.Kind == kind && target.ID == id {
			return target, true
		}
	}
	return ArtifactRef{}, false
}

func targetsOfKind(step WorkflowStep, kind ArtifactKind) []ArtifactRef {
	targets := []ArtifactRef{}
	for _, target := range step.Targets {
		if target.Kind == kind {
			targets = append(targets, target)
		}
	}
	return targets
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
}

func toolFailure(code, summary string, retryable bool) ToolResult {
	return ToolResult{
		OK: false, Summary: summary, Artifacts: []ArtifactRef{}, Retryable: retryable,
		Issues: []Issue{{Code: code, Severity: SeverityError, Evidence: summary, Verifier: "tool"}},
	}
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func prettyJSON(value any) []byte {
	raw, _ := json.MarshalIndent(value, "", "  ")
	return raw
}

func findSlide(slides []blueprint.Slide, id string) (blueprint.Slide, bool) {
	for _, slide := range slides {
		if slide.SlideID == id {
			return slide, true
		}
	}
	return blueprint.Slide{}, false
}

func firstValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
