package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const maxInlineHTMLBytes = 48 * 1024

type pptReadTool struct{ pack contextengine.ContextPack }

func (pptReadTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "read_ppt", Description: "Read the authorized global PPT model or one stable slide, preferring staged content.",
		Parameters: objectSchema([]string{"target"}, map[string]any{
			"target": targetSchema(),
			"include": map[string]any{
				"type": "array", "minItems": 1, "uniqueItems": true,
				"items": map[string]any{"type": "string", "enum": []string{"model", "html"}},
			},
		}),
	}
}

func (t pptReadTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	target, err := parseTarget(input.Args)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	if !input.Scope.AllowsRead(target) {
		return failedToolResult(ErrTargetOutOfScope.Error(), "requested read target is outside the current run scope", false)
	}
	include, err := parseIncludes(input.Args, target, t.pack.WorkSpec.Target.Artifact)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	refs, err := refsForTarget(t.pack, target)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	byKind := map[ArtifactKind]ArtifactRef{}
	for _, ref := range refs {
		byKind[ref.Kind] = ref
	}
	content := map[string]any{}
	source := "committed"
	revision := 0
	materialization := any(nil)
	for _, field := range include {
		switch field {
		case "model":
			if target.Type == "global" && t.pack.WorkSpec.Target.Artifact == model.ArtifactPresentation {
				envelope := map[string]any{}
				revisions := map[string]int{}
				for name, ref := range map[string]ArtifactRef{"deck": byKind[ArtifactDeck], "design": byKind[ArtifactDesign]} {
					raw, view, readErr := readArtifact(input.ProjectDir, input.Transaction, ref)
					if readErr != nil {
						return readFailure(readErr)
					}
					var decoded any
					if json.Unmarshal(raw, &decoded) != nil {
						return failedToolResult(CodeModelInvalid, "stored PPT model is invalid", false)
					}
					envelope[name] = decoded
					revisions[name] = revisionFromModel(raw)
					if view == "staged" {
						source = view
					}
				}
				content["model"] = envelope
				content["revisions"] = revisions
				revision = maxInt(revisions["deck"], revisions["design"])
				continue
			}
			var ref ArtifactRef
			switch {
			case target.Type == "global" && t.pack.WorkSpec.Target.Artifact == model.ArtifactBlueprint:
				ref = byKind[ArtifactDeck]
			case target.Type == "global":
				ref = byKind[ArtifactDesign]
			default:
				ref = byKind[ArtifactSlide]
			}
			raw, view, readErr := readArtifact(input.ProjectDir, input.Transaction, ref)
			if readErr != nil {
				return readFailure(readErr)
			}
			var decoded any
			if json.Unmarshal(raw, &decoded) != nil {
				return failedToolResult(CodeModelInvalid, "stored PPT model is invalid", false)
			}
			content["model"] = decoded
			revision = revisionFromModel(raw)
			if view == "staged" {
				source = view
			}
		case "html":
			ref := byKind[ArtifactPresentation]
			raw, view, readErr := readArtifact(input.ProjectDir, input.Transaction, ref)
			if readErr != nil {
				return readFailure(readErr)
			}
			inlineLimit := htmlInlineLimit(t.pack)
			if len(raw) <= inlineLimit {
				content["html"] = string(raw)
			} else {
				content["html_summary"] = map[string]any{
					"bytes": len(raw), "sha256": hashBytes(raw),
					"snippet":   string(raw[:inlineLimit]),
					"truncated": true,
				}
				if ref := contextRefForSlide(t.pack, target.SlideID); ref != nil {
					content["html_ref"] = ref.ID
				}
			}
			if view == "staged" {
				source = view
			}
			if revision == 0 {
				revision = t.pack.Revisions.Presentations[target.SlideID]
			}
		}
	}
	if target.Type == "slide" {
		materialization = t.pack.Target.Materialization
	}
	hash, _ := readTargetHash(t.pack, input, target)
	status := materializationStatus(materialization)
	if source == "staged" {
		status = "staged"
	}
	result := SuccessfulToolResult("PPT target read")
	result.Data = map[string]any{
		"target": target, "revision": revision, "hash": hash, "content": content,
		"materialization": materialization, "status": status,
		"source": source,
	}
	return result
}

func htmlInlineLimit(pack contextengine.ContextPack) int {
	remainingTokens := pack.Manifest.BudgetTokens - pack.Manifest.EstimatedTokens
	if remainingTokens <= 0 {
		return 1024
	}
	limit := remainingTokens * 3
	if limit < 1024 {
		return 1024
	}
	if limit > maxInlineHTMLBytes {
		return maxInlineHTMLBytes
	}
	return limit
}

func parseIncludes(args map[string]any, target TargetRef, artifact model.Artifact) ([]string, error) {
	values, exists := args["include"]
	if !exists {
		if target.Type == "slide" && artifact == model.ArtifactPresentation {
			return []string{"model", "html"}, nil
		}
		return []string{"model"}, nil
	}
	raw, ok := values.([]any)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("include must be a non-empty array")
	}
	out, seen := []string{}, map[string]bool{}
	for _, value := range raw {
		field := stringValue(value)
		if field != "model" && field != "html" {
			return nil, fmt.Errorf("include only accepts model and html")
		}
		if field == "html" && target.Type == "global" {
			return nil, fmt.Errorf("global target does not expose HTML")
		}
		if field == "html" && artifact == model.ArtifactBlueprint {
			return nil, fmt.Errorf("blueprint target does not expose HTML")
		}
		if seen[field] {
			return nil, fmt.Errorf("include values must be unique")
		}
		seen[field] = true
		out = append(out, field)
	}
	return out, nil
}

func contextRefForSlide(pack contextengine.ContextPack, slideID string) *contextengine.ContextRef {
	for index := range pack.Manifest.Refs {
		ref := &pack.Manifest.Refs[index]
		if ref.Kind == contextengine.RefPresentationHTML && ref.TargetID == slideID {
			return ref
		}
	}
	return nil
}

func readFailure(err error) ToolResult {
	if errorsIsNotExist(err) {
		return failedToolResult(CodeTargetNotFound, "PPT target was not found", false)
	}
	return failedToolResult("READ_FAILED", err.Error(), true)
}

func readTargetHash(pack contextengine.ContextPack, input DomainToolInput, target TargetRef) (string, error) {
	if input.Transaction != nil {
		return targetHash(pack, input.Transaction, target)
	}
	refs, err := refsForTarget(pack, target)
	if err != nil {
		return "", err
	}
	parts := []byte{}
	for _, ref := range refs {
		raw, _, readErr := readArtifact(input.ProjectDir, nil, ref)
		if readErr != nil {
			return "", readErr
		}
		parts = append(parts, []byte(ref.Key())...)
		parts = append(parts, 0)
		parts = append(parts, raw...)
		parts = append(parts, 0)
	}
	return hashBytes(parts), nil
}

func materializationStatus(value any) string {
	if value == nil {
		return "available"
	}
	if current, ok := value.(*blueprint.Materialization); ok && current != nil {
		return current.State
	}
	return "available"
}

type pptWriteTool struct{ pack contextengine.ContextPack }

func (pptWriteTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "write_ppt", Description: "Create or fully replace one authorized global or slide target in the run staging transaction.",
		Parameters: objectSchema([]string{"target", "content"}, map[string]any{
			"target": targetSchema(),
			"content": objectSchema([]string{"model"}, map[string]any{
				"model": map[string]any{"type": "object"},
				"html":  map[string]any{"type": "string", "maxLength": 2 * 1024 * 1024},
			}),
		}),
	}
}

func (t pptWriteTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	target, err := parseTarget(input.Args)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	if !input.Scope.Allows(target) {
		return failedToolResult(ErrTargetOutOfScope.Error(), "requested write target is outside the current run scope", false)
	}
	if input.Transaction == nil {
		return failedToolResult(CodeStagingRequired, "write_ppt requires run staging", false)
	}
	content, err := contentMap(input.Args)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	if t.pack.WorkSpec.Target.Artifact == model.ArtifactBlueprint {
		if _, supplied := content["html"]; supplied {
			return failedToolResult(CodeCapabilityDenied(), "blueprint targets cannot write HTML", false)
		}
	}
	if target.Type == "global" {
		if _, supplied := content["html"]; supplied {
			return failedToolResult(CodeModelInvalid, "global target cannot carry page HTML", false)
		}
		return t.writeGlobal(input, target, content)
	}
	return t.writeSlide(input, target, content)
}

func (t pptWriteTool) writeGlobal(input DomainToolInput, target TargetRef, content map[string]any) ToolResult {
	if t.pack.WorkSpec.Target.Artifact == model.ArtifactPresentation {
		return t.writePresentationGlobal(input, target, content)
	}
	refs, _ := refsForTarget(t.pack, target)
	ref := refs[0]
	modelValue, ok := content["model"]
	if !ok {
		return failedToolResult(CodeModelInvalid, "content.model is required", false)
	}
	raw, revision, err := normalizeModel(t.pack, input.Transaction, ref, modelValue)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), true)
	}
	change, err := input.Transaction.Stage(ref, "write_ppt", raw)
	if err != nil {
		return stagingFailure(err)
	}
	hash := hashBytes(raw)
	result := SuccessfulToolResult("global PPT model staged")
	result.ChangedTargets = []ChangedTarget{{Type: "global", Revision: revision, Hash: hash, Fields: []string{"model"}}}
	result.InvalidatedTargets = []TargetRef{target}
	result.Evidence = []Evidence{schemaEvidence(target, hash)}
	if ref.Kind == ArtifactDeck {
		if referenceHash, refErr := validateReferences(t.pack, input.Transaction); refErr == nil {
			result.Evidence = append(result.Evidence, referenceEvidence(referenceHash))
		} else {
			result.Issues = append(result.Issues, Issue{
				Code: "REFERENCE_INCOMPLETE", Severity: SeverityWarning, Target: target,
				Summary: refErr.Error(), Action: "write the declared slides before finish",
			})
		}
	} else {
		deck, deckErr := currentDeck(t.pack, input.Transaction)
		if deckErr == nil {
			for _, slideID := range deck.SlideOrder {
				slideTarget := TargetRef{Type: "slide", SlideID: slideID}
				result.InvalidatedTargets = append(result.InvalidatedTargets, slideTarget)
				modelRaw, _, modelErr := readArtifact(input.ProjectDir, input.Transaction, blueprintSlideRef(slideID))
				htmlRaw, _, htmlErr := readArtifact(input.ProjectDir, input.Transaction, presentationSlideRef(slideID))
				renderHash, hashErr := renderSourceHash(t.pack, input.Transaction, slideID)
				if modelErr == nil {
					var slide blueprint.Slide
					if json.Unmarshal(modelRaw, &slide) == nil && blueprint.ValidateSlide(slide) == nil {
						result.Evidence = append(result.Evidence, schemaEvidence(slideTarget, hashBytes(modelRaw)))
					}
				}
				if htmlErr == nil && hashErr == nil {
					if _, validateErr := validateHTML(htmlRaw); validateErr == nil {
						result.Evidence = append(result.Evidence, staticEvidence(slideTarget, renderHash))
					}
				}
			}
		}
	}
	result.Data = map[string]any{"operation": operationFor(change), "revision": revision, "hash": hash}
	return result
}

func (t pptWriteTool) writePresentationGlobal(input DomainToolInput, target TargetRef, content map[string]any) ToolResult {
	envelope, ok := content["model"].(map[string]any)
	if !ok {
		return failedToolResult(CodeModelInvalid, "presentation global model must contain deck and design objects", false)
	}
	deckValue, deckOK := envelope["deck"]
	designValue, designOK := envelope["design"]
	if !deckOK || !designOK {
		return failedToolResult(CodeModelInvalid, "presentation global model requires deck and design", false)
	}
	deckRaw, deckRevision, err := normalizeModel(t.pack, input.Transaction, deckRef(t.pack), deckValue)
	if err != nil {
		return failedToolResult(CodeModelInvalid, "deck: "+err.Error(), true)
	}
	designRaw, designRevision, err := normalizeModel(t.pack, input.Transaction, designRef(t.pack), designValue)
	if err != nil {
		return failedToolResult(CodeModelInvalid, "design: "+err.Error(), true)
	}
	changes, err := input.Transaction.StageBatch([]StageItem{
		{Ref: deckRef(t.pack), Source: "write_ppt", Content: deckRaw},
		{Ref: designRef(t.pack), Source: "write_ppt", Content: designRaw},
	})
	if err != nil {
		return stagingFailure(err)
	}
	hash, err := targetHash(t.pack, input.Transaction, target)
	if err != nil {
		return stagingFailure(err)
	}
	result := SuccessfulToolResult("presentation global deck and design staged atomically")
	result.ChangedTargets = []ChangedTarget{{
		Type: "global", Revision: maxInt(deckRevision, designRevision), Hash: hash, Fields: []string{"model"},
	}}
	result.InvalidatedTargets = []TargetRef{target}
	result.Evidence = []Evidence{
		schemaEvidence(target, hashBytes(deckRaw)),
		schemaEvidence(target, hashBytes(designRaw)),
	}
	if referenceHash, refErr := validateReferences(t.pack, input.Transaction); refErr == nil {
		result.Evidence = append(result.Evidence, referenceEvidence(referenceHash))
	} else {
		result.Issues = append(result.Issues, Issue{
			Code: "REFERENCE_INCOMPLETE", Severity: SeverityWarning, Target: target,
			Summary: refErr.Error(), Action: "write every slide declared by the global model before finish",
		})
	}
	var deck blueprint.Deck
	_ = json.Unmarshal(deckRaw, &deck)
	for _, slideID := range deck.SlideOrder {
		slideTarget := TargetRef{Type: "slide", SlideID: slideID}
		result.InvalidatedTargets = append(result.InvalidatedTargets, slideTarget)
		modelRaw, _, modelErr := readArtifact(input.ProjectDir, input.Transaction, blueprintSlideRef(slideID))
		htmlRaw, _, htmlErr := readArtifact(input.ProjectDir, input.Transaction, presentationSlideRef(slideID))
		renderHash, hashErr := renderSourceHash(t.pack, input.Transaction, slideID)
		if modelErr == nil {
			result.Evidence = append(result.Evidence, schemaEvidence(slideTarget, hashBytes(modelRaw)))
		}
		if htmlErr == nil && hashErr == nil {
			if _, validateErr := validateHTML(htmlRaw); validateErr == nil {
				result.Evidence = append(result.Evidence, staticEvidence(slideTarget, renderHash))
			}
		}
	}
	result.Data = map[string]any{
		"operation": operationsFor(changes), "revision": map[string]int{
			"deck": deckRevision, "design": designRevision,
		},
		"hash": hash, "deck_hash": hashBytes(deckRaw), "design_hash": hashBytes(designRaw),
	}
	return result
}

func (t pptWriteTool) writeSlide(input DomainToolInput, target TargetRef, content map[string]any) ToolResult {
	modelValue, ok := content["model"]
	if !ok {
		return failedToolResult(CodeModelInvalid, "content.model is required", false)
	}
	modelRaw, revision, err := normalizeModel(t.pack, input.Transaction, blueprintSlideRef(target.SlideID), modelValue)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), true)
	}
	var slide blueprint.Slide
	_ = json.Unmarshal(modelRaw, &slide)
	if err := validateSlideReference(t.pack, input.Transaction, slide); err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), true)
	}
	items := []StageItem{{Ref: blueprintSlideRef(target.SlideID), Source: "write_ppt", Content: modelRaw}}
	fields := []string{"model"}
	if t.pack.WorkSpec.Target.Artifact == model.ArtifactPresentation {
		html, ok := content["html"].(string)
		if !ok || strings.TrimSpace(html) == "" {
			return failedToolResult(CodeModelInvalid, "presentation slide requires content.html", false)
		}
		if issues, htmlErr := validateHTML([]byte(html)); htmlErr != nil {
			result := failedToolResult(CodeModelInvalid, htmlErr.Error(), true)
			result.Issues = issues
			return result
		}
		items = append(items, StageItem{Ref: presentationSlideRef(target.SlideID), Source: "write_ppt", Content: []byte(html)})
		fields = append(fields, "html")
	}
	changes, err := input.Transaction.StageBatch(items)
	if err != nil {
		return stagingFailure(err)
	}
	hash, err := targetHash(t.pack, input.Transaction, target)
	if err != nil {
		return failedToolResult("STAGING_FAILED", err.Error(), true)
	}
	result := SuccessfulToolResult("slide staged with consistent model and HTML")
	result.ChangedTargets = []ChangedTarget{{Type: "slide", SlideID: target.SlideID, Revision: revision, Hash: hash, Fields: fields}}
	result.InvalidatedTargets = []TargetRef{target}
	result.Evidence = []Evidence{schemaEvidence(target, hashBytes(modelRaw))}
	if t.pack.WorkSpec.Target.Artifact == model.ArtifactPresentation {
		renderHash, _ := renderSourceHash(t.pack, input.Transaction, target.SlideID)
		result.Evidence = append(result.Evidence, staticEvidence(target, renderHash))
	}
	if referenceHash, refErr := validateReferences(t.pack, input.Transaction); refErr == nil {
		result.Evidence = append(result.Evidence, referenceEvidence(referenceHash))
	}
	result.Data = map[string]any{
		"operation": operationsFor(changes), "revision": revision, "hash": hash,
		"model_hash": hashBytes(modelRaw),
	}
	return result
}

func operationFor(change ArtifactChange) string {
	if change.BeforeHash == hashBytes(nil) {
		return "create"
	}
	return "replace"
}

func operationsFor(changes []ArtifactChange) []string {
	out := make([]string, len(changes))
	for index, change := range changes {
		out[index] = operationFor(change)
	}
	return out
}

func CodeCapabilityDenied() string { return ErrCapabilityDenied.Error() }

type pptEditTool struct{ pack contextengine.ContextPack }

func (pptEditTool) Schema() ToolSchema {
	editSchema := map[string]any{
		"oneOf": []any{
			objectSchema([]string{"type", "op", "path"}, map[string]any{
				"type":  map[string]any{"const": "json_edit"},
				"op":    map[string]any{"type": "string", "enum": []string{"add", "replace", "remove"}},
				"path":  map[string]any{"type": "string", "pattern": "^/"},
				"value": map[string]any{},
			}),
			objectSchema([]string{"type", "field", "old_text", "new_text"}, map[string]any{
				"type":     map[string]any{"const": "text_edit"},
				"field":    map[string]any{"const": "html"},
				"old_text": map[string]any{"type": "string", "minLength": 1},
				"new_text": map[string]any{"type": "string"},
			}),
		},
	}
	return ToolSchema{
		Name: "edit_ppt", Description: "Atomically apply schema-constrained JSON edits or uniquely anchored HTML text edits.",
		Parameters: objectSchema([]string{"target", "edits"}, map[string]any{
			"target": targetSchema(),
			"edits":  map[string]any{"type": "array", "minItems": 1, "maxItems": 32, "items": editSchema},
		}),
	}
}

func (t pptEditTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	target, err := parseTarget(input.Args)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	if !input.Scope.Allows(target) {
		return failedToolResult(ErrTargetOutOfScope.Error(), "requested edit target is outside the current run scope", false)
	}
	if input.Transaction == nil {
		return failedToolResult(CodeStagingRequired, "edit_ppt requires run staging", false)
	}
	values, ok := input.Args["edits"].([]any)
	if !ok || len(values) == 0 {
		return failedToolResult(CodeModelInvalid, "at least one edit is required", true)
	}
	if len(values) > 32 {
		return failedToolResult(CodeModelInvalid, "large reconstructions must use write_ppt", false)
	}
	return t.applyEdits(input, target, values)
}

func (t pptEditTool) applyEdits(input DomainToolInput, target TargetRef, values []any) ToolResult {
	if target.Type == "global" && t.pack.WorkSpec.Target.Artifact == model.ArtifactPresentation {
		return t.applyPresentationGlobalEdits(input, target, values)
	}
	refs, err := refsForTarget(t.pack, target)
	if err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	var modelRef, htmlRef ArtifactRef
	for _, ref := range refs {
		if ref.Kind == ArtifactPresentation {
			htmlRef = ref
		} else {
			modelRef = ref
		}
	}
	modelRaw, _, err := readArtifact(input.ProjectDir, input.Transaction, modelRef)
	if err != nil {
		return readFailure(err)
	}
	var modelDoc any
	if err := json.Unmarshal(modelRaw, &modelDoc); err != nil {
		return failedToolResult(CodeModelInvalid, err.Error(), false)
	}
	htmlRaw := []byte(nil)
	if htmlRef.Kind != "" {
		htmlRaw, _, err = readArtifact(input.ProjectDir, input.Transaction, htmlRef)
		if err != nil {
			return readFailure(err)
		}
	}
	modelChanged, htmlChanged := false, false
	for _, value := range values {
		edit, ok := value.(map[string]any)
		if !ok {
			return failedToolResult(CodeModelInvalid, "each edit must be an object", false)
		}
		switch stringValue(edit["type"]) {
		case "json_edit":
			if err := applyJSONEdit(&modelDoc, stringValue(edit["op"]), stringValue(edit["path"]), edit["value"]); err != nil {
				return failedToolResult(CodeModelInvalid, err.Error(), true)
			}
			modelChanged = true
		case "text_edit":
			if htmlRef.Kind == "" || stringValue(edit["field"]) != "html" {
				return failedToolResult(ErrCapabilityDenied.Error(), "HTML edits are only allowed for presentation slides", false)
			}
			oldText, newText := stringValue(edit["old_text"]), stringValue(edit["new_text"])
			count := strings.Count(string(htmlRaw), oldText)
			if oldText == "" || count == 0 {
				return failedToolResult(CodeEditAnchorNotFound, "text edit anchor was not found", true)
			}
			if count > 1 {
				return failedToolResult(CodeEditAnchorAmbiguous, "text edit anchor matched more than once", true)
			}
			htmlRaw = []byte(strings.Replace(string(htmlRaw), oldText, newText, 1))
			htmlChanged = true
		default:
			return failedToolResult(CodeModelInvalid, "unsupported edit type", false)
		}
	}
	nextModelRaw := modelRaw
	revision := revisionFromModel(modelRaw)
	if modelChanged {
		nextModelValue := modelDoc
		nextModelRaw, revision, err = normalizeModel(t.pack, input.Transaction, modelRef, nextModelValue)
		if err != nil {
			return failedToolResult(CodeModelInvalid, err.Error(), true)
		}
	}
	if modelChanged && modelRef.Kind == ArtifactSlide {
		var slide blueprint.Slide
		_ = json.Unmarshal(nextModelRaw, &slide)
		if err := validateSlideReference(t.pack, input.Transaction, slide); err != nil {
			return failedToolResult(CodeModelInvalid, err.Error(), true)
		}
	}
	if htmlRef.Kind != "" {
		if issues, htmlErr := validateHTML(htmlRaw); htmlErr != nil {
			result := failedToolResult(CodeModelInvalid, htmlErr.Error(), true)
			result.Issues = issues
			return result
		}
	}
	items := []StageItem{}
	fields := []string{}
	if modelChanged {
		items = append(items, StageItem{Ref: modelRef, Source: "edit_ppt", Content: nextModelRaw})
		fields = append(fields, "model")
	}
	if htmlChanged {
		items = append(items, StageItem{Ref: htmlRef, Source: "edit_ppt", Content: htmlRaw})
		fields = append(fields, "html")
	}
	if len(items) == 0 {
		return failedToolResult(CodeModelInvalid, "edits produced no change", false)
	}
	if _, err := input.Transaction.StageBatch(items); err != nil {
		return stagingFailure(err)
	}
	hash, err := targetHash(t.pack, input.Transaction, target)
	if err != nil {
		return failedToolResult("STAGING_FAILED", err.Error(), true)
	}
	result := SuccessfulToolResult("PPT target edited atomically")
	result.ChangedTargets = []ChangedTarget{{Type: target.Type, SlideID: target.SlideID, Revision: revision, Hash: hash, Fields: fields}}
	result.InvalidatedTargets = []TargetRef{target}
	result.Evidence = append(result.Evidence, schemaEvidence(target, hashBytes(nextModelRaw)))
	if htmlRef.Kind != "" {
		renderHash, _ := renderSourceHash(t.pack, input.Transaction, target.SlideID)
		result.Evidence = append(result.Evidence, staticEvidence(target, renderHash))
	}
	if modelRef.Kind == ArtifactDesign {
		if deck, deckErr := currentDeck(t.pack, input.Transaction); deckErr == nil {
			for _, slideID := range deck.SlideOrder {
				slideTarget := TargetRef{Type: "slide", SlideID: slideID}
				result.InvalidatedTargets = append(result.InvalidatedTargets, slideTarget)
				modelRaw, _, modelErr := readArtifact(input.ProjectDir, input.Transaction, blueprintSlideRef(slideID))
				htmlRaw, _, htmlErr := readArtifact(input.ProjectDir, input.Transaction, presentationSlideRef(slideID))
				renderHash, hashErr := renderSourceHash(t.pack, input.Transaction, slideID)
				if modelErr == nil {
					result.Evidence = append(result.Evidence, schemaEvidence(slideTarget, hashBytes(modelRaw)))
				}
				if htmlErr == nil && hashErr == nil {
					if _, validateErr := validateHTML(htmlRaw); validateErr == nil {
						result.Evidence = append(result.Evidence, staticEvidence(slideTarget, renderHash))
					}
				}
			}
		}
	}
	if referenceHash, refErr := validateReferences(t.pack, input.Transaction); refErr == nil {
		result.Evidence = append(result.Evidence, referenceEvidence(referenceHash))
	}
	result.Data = map[string]any{"revision": revision, "hash": hash, "fields": fields}
	return result
}

func (t pptEditTool) applyPresentationGlobalEdits(input DomainToolInput, target TargetRef, values []any) ToolResult {
	deckRaw, _, err := readArtifact(input.ProjectDir, input.Transaction, deckRef(t.pack))
	if err != nil {
		return readFailure(err)
	}
	designRaw, _, err := readArtifact(input.ProjectDir, input.Transaction, designRef(t.pack))
	if err != nil {
		return readFailure(err)
	}
	var deckDoc, designDoc any
	if json.Unmarshal(deckRaw, &deckDoc) != nil || json.Unmarshal(designRaw, &designDoc) != nil {
		return failedToolResult(CodeModelInvalid, "stored presentation global model is invalid", false)
	}
	envelope := any(map[string]any{"deck": deckDoc, "design": designDoc})
	deckChanged, designChanged := false, false
	for _, value := range values {
		edit, ok := value.(map[string]any)
		if !ok || stringValue(edit["type"]) != "json_edit" {
			return failedToolResult(ErrCapabilityDenied.Error(), "presentation global only supports JSON edits under /deck or /design", false)
		}
		path := stringValue(edit["path"])
		switch {
		case strings.HasPrefix(path, "/deck/"):
			deckChanged = true
		case strings.HasPrefix(path, "/design/"):
			designChanged = true
		default:
			return failedToolResult(CodeModelInvalid, "presentation global JSON paths must start with /deck/ or /design/", false)
		}
		if err := applyJSONEdit(&envelope, stringValue(edit["op"]), path, edit["value"]); err != nil {
			return failedToolResult(CodeModelInvalid, err.Error(), true)
		}
	}
	updated := envelope.(map[string]any)
	items := []StageItem{}
	evidence := []Evidence{}
	deckRevision, designRevision := revisionFromModel(deckRaw), revisionFromModel(designRaw)
	if deckChanged {
		deckRaw, deckRevision, err = normalizeModel(t.pack, input.Transaction, deckRef(t.pack), updated["deck"])
		if err != nil {
			return failedToolResult(CodeModelInvalid, "deck: "+err.Error(), true)
		}
		items = append(items, StageItem{Ref: deckRef(t.pack), Source: "edit_ppt", Content: deckRaw})
		evidence = append(evidence, schemaEvidence(target, hashBytes(deckRaw)))
	}
	if designChanged {
		designRaw, designRevision, err = normalizeModel(t.pack, input.Transaction, designRef(t.pack), updated["design"])
		if err != nil {
			return failedToolResult(CodeModelInvalid, "design: "+err.Error(), true)
		}
		items = append(items, StageItem{Ref: designRef(t.pack), Source: "edit_ppt", Content: designRaw})
		evidence = append(evidence, schemaEvidence(target, hashBytes(designRaw)))
	}
	if _, err := input.Transaction.StageBatch(items); err != nil {
		return stagingFailure(err)
	}
	hash, err := targetHash(t.pack, input.Transaction, target)
	if err != nil {
		return stagingFailure(err)
	}
	result := SuccessfulToolResult("presentation global model edited atomically")
	result.ChangedTargets = []ChangedTarget{{
		Type: "global", Revision: maxInt(deckRevision, designRevision), Hash: hash, Fields: []string{"model"},
	}}
	result.InvalidatedTargets = []TargetRef{target}
	result.Evidence = evidence
	if referenceHash, refErr := validateReferences(t.pack, input.Transaction); refErr == nil {
		result.Evidence = append(result.Evidence, referenceEvidence(referenceHash))
	}
	if designChanged {
		if deck, deckErr := currentDeck(t.pack, input.Transaction); deckErr == nil {
			for _, slideID := range deck.SlideOrder {
				slideTarget := TargetRef{Type: "slide", SlideID: slideID}
				result.InvalidatedTargets = append(result.InvalidatedTargets, slideTarget)
				modelRaw, _, modelErr := readArtifact(input.ProjectDir, input.Transaction, blueprintSlideRef(slideID))
				htmlRaw, _, htmlErr := readArtifact(input.ProjectDir, input.Transaction, presentationSlideRef(slideID))
				renderHash, hashErr := renderSourceHash(t.pack, input.Transaction, slideID)
				if modelErr == nil {
					result.Evidence = append(result.Evidence, schemaEvidence(slideTarget, hashBytes(modelRaw)))
				}
				if htmlErr == nil && hashErr == nil {
					if _, validateErr := validateHTML(htmlRaw); validateErr == nil {
						result.Evidence = append(result.Evidence, staticEvidence(slideTarget, renderHash))
					}
				}
			}
		}
	}
	result.Data = map[string]any{
		"revision": map[string]int{"deck": deckRevision, "design": designRevision},
		"hash":     hash, "fields": []string{"model"},
	}
	return result
}
