package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const maxPPTContentBytes = 2 * 1024 * 1024

type pptReadTool struct{ pack contextengine.ContextPack }

func (pptReadTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "read_ppt",
		Description: "Read one disclosed PPT resource. Returns the complete saved JSON or HTML string.",
		Parameters: objectSchema([]string{"resource"}, map[string]any{
			"resource": resourceSchema(),
		}),
	}
}

func (t pptReadTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	resource, err := parseResource(input.Args)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	if !input.Scope.AllowsRead(resource) {
		return failedToolResult(CodeTargetOutOfScope, "requested resource is outside the current run scope", false)
	}
	ref, err := refForResource(t.pack, resource)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	raw, _, err := readArtifact(input.ProjectDir, input.Transaction, ref)
	if err != nil {
		return readFailure(err)
	}
	if len(raw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "resource exceeds the read_ppt content limit", false)
	}
	result := SuccessfulToolResult("resource read")
	result.Observation = string(raw)
	return result
}

type pptWriteTool struct{ pack contextengine.ContextPack }

func (pptWriteTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "write_ppt",
		Description: "Create or fully replace one disclosed PPT resource in run staging. content is always a string.",
		Parameters: objectSchema([]string{"resource", "content"}, map[string]any{
			"resource": resourceSchema(),
			"content":  map[string]any{"type": "string", "maxLength": maxPPTContentBytes},
		}),
	}
}

func (t pptWriteTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	resource, err := parseResource(input.Args)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	if !input.Scope.Allows(resource) {
		return failedToolResult(CodeTargetOutOfScope, "requested resource is outside the current run scope", false)
	}
	if input.Transaction == nil {
		return failedToolResult(CodeStagingRequired, "write_ppt requires run staging", false)
	}
	content, ok := input.Args["content"].(string)
	if !ok {
		return failedToolResult(CodeContentInvalid, "content must be a string", false)
	}
	if len(content) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "content exceeds the write_ppt limit", false)
	}
	ref, err := refForResource(t.pack, resource)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	raw, revision, issues, err := normalizeResource(t.pack, input.Transaction, ref, []byte(content))
	if err != nil {
		result := failedToolResult(CodeContentInvalid, err.Error(), true)
		if len(issues) > 0 {
			result.Issues = issues
		}
		return result
	}
	if ref.Kind == ArtifactSlideSpec {
		var slide spec.SlideSpec
		_ = json.Unmarshal(raw, &slide)
		if err := validateSlideReference(t.pack, input.Transaction, slide); err != nil {
			return failedToolResult(CodeContentInvalid, err.Error(), true)
		}
	}
	change, err := stagePPTMutation(t.pack, input.Transaction, ref, "write_ppt", raw)
	if err != nil {
		return stagingFailure(err)
	}
	return mutationResult(t.pack, input, resource, ref, change, raw, revision, "resource staged")
}

type pptEditTool struct{ pack contextengine.ContextPack }

func (pptEditTool) Schema() ToolSchema {
	edit := objectSchema([]string{"old_text", "new_text"}, map[string]any{
		"old_text": map[string]any{"type": "string", "minLength": 1},
		"new_text": map[string]any{"type": "string"},
	})
	return ToolSchema{
		Name:        "edit_ppt",
		Description: "Atomically apply ordered, uniquely anchored exact text replacements to one PPT resource.",
		Parameters: objectSchema([]string{"resource", "edits"}, map[string]any{
			"resource": resourceSchema(),
			"edits":    map[string]any{"type": "array", "minItems": 1, "maxItems": 32, "items": edit},
		}),
	}
}

func (t pptEditTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	resource, err := parseResource(input.Args)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	if !input.Scope.Allows(resource) {
		return failedToolResult(CodeTargetOutOfScope, "requested resource is outside the current run scope", false)
	}
	if input.Transaction == nil {
		return failedToolResult(CodeStagingRequired, "edit_ppt requires run staging", false)
	}
	values, ok := input.Args["edits"].([]any)
	if !ok || len(values) == 0 || len(values) > 32 {
		return failedToolResult(CodeContentInvalid, "edits must contain between 1 and 32 replacements", false)
	}
	ref, err := refForResource(t.pack, resource)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	original, _, err := readArtifact(input.ProjectDir, input.Transaction, ref)
	if err != nil {
		return readFailure(err)
	}
	candidate := string(original)
	for index, value := range values {
		edit, ok := value.(map[string]any)
		if !ok {
			return failedToolResult(CodeContentInvalid, fmt.Sprintf("edit %d must be an object", index+1), false)
		}
		oldText, oldOK := edit["old_text"].(string)
		newText, newOK := edit["new_text"].(string)
		if !oldOK || oldText == "" || !newOK {
			return failedToolResult(CodeContentInvalid, fmt.Sprintf("edit %d requires string old_text and new_text", index+1), false)
		}
		switch count := strings.Count(candidate, oldText); {
		case count == 0:
			return failedToolResult(CodeEditAnchorNotFound, fmt.Sprintf("edit %d anchor was not found", index+1), true)
		case count > 1:
			return failedToolResult(CodeEditAnchorAmbiguous, fmt.Sprintf("edit %d anchor matched more than once", index+1), true)
		default:
			candidate = strings.Replace(candidate, oldText, newText, 1)
		}
	}
	if candidate == string(original) {
		return failedToolResult(CodeContentInvalid, "edits produced no change", false)
	}
	if len(candidate) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "edited content exceeds the limit", false)
	}
	raw, revision, issues, err := normalizeResource(t.pack, input.Transaction, ref, []byte(candidate))
	if err != nil {
		result := failedToolResult(CodeContentInvalid, err.Error(), true)
		if len(issues) > 0 {
			result.Issues = issues
		}
		return result
	}
	if ref.Kind == ArtifactSlideSpec {
		var slide spec.SlideSpec
		_ = json.Unmarshal(raw, &slide)
		if err := validateSlideReference(t.pack, input.Transaction, slide); err != nil {
			return failedToolResult(CodeContentInvalid, err.Error(), true)
		}
	}
	change, err := stagePPTMutation(t.pack, input.Transaction, ref, "edit_ppt", raw)
	if err != nil {
		return stagingFailure(err)
	}
	return mutationResult(t.pack, input, resource, ref, change, raw, revision, "resource edited atomically")
}

func stagePPTMutation(
	pack contextengine.ContextPack,
	tx *RunSession,
	ref ArtifactRef,
	source string,
	raw []byte,
) (ArtifactChange, error) {
	if ref.Kind != ArtifactDesign {
		return tx.Stage(ref, source, raw)
	}
	var design spec.Design
	if err := json.Unmarshal(raw, &design); err != nil {
		return ArtifactChange{}, err
	}
	changes, err := tx.StageBatch([]StageItem{
		{Ref: ref, Source: source, Content: raw},
		{Ref: designTokensRef(pack), Source: "runtime:design-tokens", Content: spec.DesignTokensCSS(design)},
	})
	if err != nil {
		return ArtifactChange{}, err
	}
	return changes[0], nil
}

func normalizeResource(
	pack contextengine.ContextPack,
	tx *RunSession,
	ref ArtifactRef,
	content []byte,
) ([]byte, int, []Issue, error) {
	if ref.Kind == ArtifactSlideHTML {
		issues, err := validateHTML(content)
		return content, 0, issues, err
	}
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		return nil, 0, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	raw, revision, err := normalizeModel(pack, tx, ref, decoded)
	return raw, revision, nil, err
}

func mutationResult(
	pack contextengine.ContextPack,
	input DomainToolInput,
	resource Resource,
	ref ArtifactRef,
	change ArtifactChange,
	raw []byte,
	revision int,
	summary string,
) ToolResult {
	result := SuccessfulToolResult(summary)
	result.ChangedTargets = []ChangedTarget{{
		Type: resource.Type, SlideID: resource.SlideID, Part: resource.Part,
		Revision: revision, Hash: hashBytes(raw),
	}}
	result.InvalidatedTargets = invalidatedByMutation(pack, input.Transaction, resource)
	switch ref.Kind {
	case ArtifactOutline, ArtifactDesign, ArtifactSlideSpec:
		result.Evidence = append(result.Evidence, schemaEvidence(resource, hashBytes(raw)))
	case ArtifactSlideHTML:
		if renderHash, err := renderSourceHash(pack, input.Transaction, resource.SlideID); err == nil {
			result.Evidence = append(result.Evidence, staticEvidence(resource, renderHash))
		}
	}
	if referenceHash, err := validateReferences(pack, input.Transaction); err == nil {
		result.Evidence = append(result.Evidence, referenceEvidence(referenceHash))
	}
	result.Data = map[string]any{
		"operation": operationFor(change), "resource": resource, "revision": revision,
	}
	return result
}

func invalidatedByMutation(pack contextengine.ContextPack, tx *RunSession, resource Resource) []Resource {
	out := []Resource{resource}
	switch {
	case resource == (Resource{Type: "deck", Part: "design"}):
		if outline, err := currentOutline(pack, tx); err == nil {
			for _, slideID := range outline.SlideOrder {
				out = append(out, Resource{Type: "slide", SlideID: slideID, Part: "html"})
			}
		}
	case resource.Type == "slide" && resource.Part == "spec":
		out = append(out, Resource{Type: "slide", SlideID: resource.SlideID, Part: "html"})
	}
	return uniqueTargets(out)
}

func operationFor(change ArtifactChange) string {
	if change.BeforeHash == hashBytes(nil) {
		return "create"
	}
	return "replace"
}
