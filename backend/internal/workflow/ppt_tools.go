package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const maxPPTContentBytes = 2 * 1024 * 1024

type pptReadTool struct{ pack contextengine.ContextPack }

func (pptReadTool) Schema() ToolSchema {
	return ToolSchema{Name: "read_ppt", Description: "Read one authorized manifest, outline, design, slide spec, or slide HTML resource by structured identity.", Parameters: objectSchema([]string{"resource"}, map[string]any{"resource": resourceSchema()})}
}
func (t pptReadTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	resource, err := parseResource(input.Args)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	if !AllowsRead(input.Scope, resource) {
		return failedToolResult(CodeTargetOutOfScope, "requested resource is outside the current run scope", false)
	}
	ref, err := refForResource(t.pack, resource)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	raw, _, err := readArtifact(input.ProjectDir, input.Session, ref)
	if err != nil {
		return readFailure(err)
	}
	if len(raw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "resource exceeds the read limit", false)
	}
	result := SuccessfulToolResult("resource read")
	result.Observation = string(raw)
	return result
}

type mutatePPTTool struct{ pack contextengine.ContextPack }

func (t mutatePPTTool) Schema() ToolSchema {
	return ToolSchema{Name: "mutate_ppt", Description: "Apply exactly one typed PPT domain operation. Runtime creates every new stable ID; initialization and insertion accept client_ref only.", Parameters: mutationSchema(t.pack)}
}
func (t mutatePPTTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	if input.Session == nil {
		return failedToolResult(CodeRunSessionRequired, "mutate_ppt requires an active run session", false)
	}
	raw, err := json.Marshal(input.Args)
	if err != nil {
		return failedToolResult(CodeContentInvalid, err.Error(), false)
	}
	if len(raw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "mutation exceeds content limit", false)
	}
	var req pptmutation.Request
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&req); err != nil {
		return failedToolResult(CodeContentInvalid, err.Error(), false)
	}
	if !operationAllowed(input.Scope, req) {
		return failedToolResult(CodeTargetOutOfScope, "mutation operation is outside the current run scope", false)
	}
	buffer := pptmutation.NewBuffer(runWorkspace{session: input.Session, pack: t.pack})
	engine := pptmutation.Service{Workspace: buffer, ProjectID: t.pack.Project.ID, ValidateHTML: func(raw []byte) error {
		issues, err := validateHTML(raw)
		if err != nil {
			return fmt.Errorf("%s", issues[0].Summary)
		}
		return nil
	}}
	result, err := engine.Apply(req)
	if err != nil {
		code := CodeContentInvalid
		switch {
		case errors.Is(err, pptmutation.ErrRevisionConflict):
			code = CodeRevisionConflict
		case errors.Is(err, pptmutation.ErrPatchPathDenied):
			code = CodePatchPathDenied
		case errors.Is(err, pptmutation.ErrPatchInvalid):
			code = CodePatchInvalid
		}
		return failedToolResult(code, err.Error(), true)
	}
	if err = buffer.Commit(); err != nil {
		return writeFailure(err)
	}
	out := SuccessfulToolResult("PPT mutation applied")
	out.Data = map[string]any{"operation": result.Operation, "revisions": result.Revisions, "created": result.Created, "affected_slide_ids": result.AffectedSlideIDs, "invalidated_slide_ids": result.InvalidatedSlideIDs}
	resource := resourceForOperation(req)
	if resource.Type != "" {
		ref, _ := refForResource(t.pack, resource)
		content, _, readErr := readArtifact(input.ProjectDir, input.Session, ref)
		if readErr == nil {
			kind := "schema"
			if resource.Part == "html" {
				kind = "static"
			}
			hash := hashBytes(content)
			out.ChangedTargets = []ChangedTarget{{Type: resource.Type, SlideID: resource.SlideID, Part: resource.Part, Revision: revisionFromModel(content), Hash: hash}}
			out.Evidence = []Evidence{newEvidence(kind, resource, hash, map[string]any{"valid": true})}
		}
	}
	for _, id := range result.InvalidatedSlideIDs {
		out.InvalidatedTargets = append(out.InvalidatedTargets, Resource{Type: "slide", SlideID: id, Part: "html"})
	}
	return out
}

type runWorkspace struct {
	session *RunSession
	pack    contextengine.ContextPack
}

func (w runWorkspace) Read(path string) ([]byte, error) {
	return w.session.Read(refForPath(w.pack, path))
}
func (w runWorkspace) Write(path string, raw []byte) error {
	_, err := w.session.Write(refForPath(w.pack, path), "mutate_ppt", raw)
	return err
}
func (w runWorkspace) Delete(path string) error {
	return w.session.Delete(refForPath(w.pack, path), "mutate_ppt")
}

func refForPath(pack contextengine.ContextPack, path string) ArtifactRef {
	switch path {
	case "manifest.json":
		return manifestRef(pack)
	case "outline.json":
		return outlineRef(pack)
	case "design.json":
		return designRef(pack)
	}
	if strings.HasPrefix(path, "slides/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 3 {
			switch parts[2] {
			case "spec.json":
				return specSlideRef(parts[1])
			case "index.html":
				return slideHTMLRef(parts[1])
			case "materialization.json":
				return ArtifactRef{Kind: ArtifactDerived, ID: parts[1] + ":materialization", Path: path, Project: pack.Project.ID}
			}
		}
	}
	return ArtifactRef{Kind: ArtifactDerived, ID: path, Path: path, Project: pack.Project.ID}
}

func resourceForOperation(req pptmutation.Request) Resource {
	switch {
	case req.Op == "manifest.patch":
		return Resource{Type: "deck", Part: "manifest"}
	case strings.HasPrefix(req.Op, "outline."):
		return Resource{Type: "deck", Part: "outline"}
	case strings.HasPrefix(req.Op, "design."):
		return Resource{Type: "deck", Part: "design"}
	case strings.HasPrefix(req.Op, "slide.spec."):
		return Resource{Type: "slide", SlideID: req.SlideID, Part: "spec"}
	case strings.HasPrefix(req.Op, "slide.html."):
		return Resource{Type: "slide", SlideID: req.SlideID, Part: "html"}
	}
	return Resource{}
}
func operationAllowed(scope model.RunScope, req pptmutation.Request) bool {
	return mutationOperationAllowed(scope, req.Op, req.SlideID)
}

func mutationSchema(pack contextengine.ContextPack) map[string]any {
	patch := func(operation string) map[string]any {
		pathSchema := func(patchOp string) map[string]any {
			rules := pptmutation.PatchPathRules(operation, patchOp)
			patterns := make([]any, 0, len(rules))
			for _, rule := range rules {
				patterns = append(patterns, map[string]any{
					"type": "string", "pattern": rule.Pattern, "description": rule.Description,
				})
			}
			return map[string]any{"oneOf": patterns}
		}
		withValue := func(patchOp string) map[string]any {
			return objectSchema([]string{"op", "path", "value"}, map[string]any{
				"op": map[string]any{"const": patchOp}, "path": pathSchema(patchOp), "value": map[string]any{},
			})
		}
		remove := objectSchema([]string{"op", "path"}, map[string]any{
			"op": map[string]any{"const": "remove"}, "path": pathSchema("remove"),
		})
		return map[string]any{
			"type": "array", "minItems": 1, "maxItems": 32,
			"items": map[string]any{"oneOf": []any{withValue("add"), remove, withValue("replace")}},
		}
	}
	position := objectSchema(nil, map[string]any{"parent_id": map[string]any{"type": "string"}, "before_id": map[string]any{"type": "string"}, "after_id": map[string]any{"type": "string"}})
	variant := func(op string, required []string, props map[string]any) any {
		props["op"] = map[string]any{"const": op}
		props["expected_revision"] = map[string]any{"type": "integer", "minimum": 0}
		return objectSchema(append([]string{"op"}, required...), props)
	}
	text := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	roles := make([]string, 0, len(spec.SlideRoleValues()))
	for _, role := range spec.SlideRoleValues() {
		roles = append(roles, string(role))
	}
	role := map[string]any{"enum": roles}
	draftSlide := objectSchema([]string{"client_ref", "title", "role"}, map[string]any{"client_ref": text(120), "title": text(160), "role": role})
	draftSubsection := objectSchema([]string{"client_ref", "title", "purpose", "slides"}, map[string]any{"client_ref": text(120), "title": text(160), "purpose": text(400), "slides": map[string]any{"type": "array", "items": draftSlide}})
	draftSection := objectSchema([]string{"client_ref", "title", "purpose", "slides", "subsections"}, map[string]any{"client_ref": text(120), "title": text(160), "purpose": text(400), "slides": map[string]any{"type": "array", "items": draftSlide}, "subsections": map[string]any{"type": "array", "items": draftSubsection}})
	draftNode := map[string]any{"oneOf": []any{
		objectSchema([]string{"kind", "client_ref", "title", "purpose", "slides", "subsections"}, map[string]any{"kind": map[string]any{"const": "section"}, "client_ref": text(120), "title": text(160), "purpose": text(400), "slides": map[string]any{"type": "array", "items": draftSlide}, "subsections": map[string]any{"type": "array", "items": draftSubsection}}),
		objectSchema([]string{"kind", "client_ref", "title", "purpose"}, map[string]any{"kind": map[string]any{"const": "subsection"}, "client_ref": text(120), "title": text(160), "purpose": text(400)}),
		objectSchema([]string{"kind", "client_ref", "title", "role"}, map[string]any{"kind": map[string]any{"const": "slide"}, "client_ref": text(120), "title": text(160), "role": role}),
	}}
	changes := objectSchema(nil, map[string]any{"title": text(160), "purpose": text(400), "role": role})
	changes["minProperties"] = 1
	chrome := objectSchema([]string{"type", "placement", "style"}, map[string]any{"type": map[string]any{"enum": []string{"page_number", "section_marker", "key_message", "deck_title"}}, "placement": map[string]any{"enum": []string{"top-left", "top-center", "top-right", "bottom-left", "bottom-center", "bottom-right", "left-edge", "right-edge"}}, "style": text(160)})
	design := objectSchema([]string{"direction", "density", "chrome"}, map[string]any{"direction": text(600), "density": map[string]any{"enum": []string{"sparse", "medium", "dense"}}, "chrome": map[string]any{"type": "array", "maxItems": 12, "items": chrome}})
	element := objectSchema([]string{"type", "intent"}, map[string]any{"type": map[string]any{"enum": []string{"text", "list", "metric", "quote", "table", "chart", "diagram", "code", "asset"}}, "intent": text(1200)})
	slideSpec := objectSchema([]string{"key_message", "elements"}, map[string]any{"key_message": text(500), "elements": map[string]any{"type": "array", "items": element}, "layout": text(80)})
	edits := map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"old_text", "new_text"}, map[string]any{"old_text": text(maxPPTContentBytes), "new_text": map[string]any{"type": "string", "maxLength": maxPPTContentBytes}})}
	variants := []any{
		variant("manifest.patch", []string{"patch"}, map[string]any{"patch": patch("manifest.patch")}),
		variant("outline.init", []string{"structure"}, map[string]any{"structure": map[string]any{"type": "array", "minItems": 1, "items": draftSection}}),
		variant("outline.insert", []string{"node", "position"}, map[string]any{"node": draftNode, "position": position, "direct_slides_policy": map[string]any{"enum": []string{"move_into_new_subsection"}}}),
		variant("outline.move", []string{"node_id", "position"}, map[string]any{"node_id": map[string]any{"type": "string"}, "position": position}),
		variant("outline.update", []string{"node_id", "changes"}, map[string]any{"node_id": map[string]any{"type": "string"}, "changes": changes}),
		variant("outline.remove", []string{"node_id"}, map[string]any{"node_id": map[string]any{"type": "string"}, "child_policy": map[string]any{"enum": []string{"promote_to_section"}}}),
		variant("design.write", []string{"design"}, map[string]any{"design": design}), variant("design.patch", []string{"patch"}, map[string]any{"patch": patch("design.patch")}),
		variant("slide.spec.write", []string{"slide_id", "spec"}, map[string]any{"slide_id": slideIDSchema(pack), "spec": slideSpec}), variant("slide.spec.patch", []string{"slide_id", "patch"}, map[string]any{"slide_id": slideIDSchema(pack), "patch": patch("slide.spec.patch")}),
		variant("slide.html.write", []string{"slide_id", "html"}, map[string]any{"slide_id": slideIDSchema(pack), "html": map[string]any{"type": "string", "minLength": 1, "maxLength": maxPPTContentBytes}}), variant("slide.html.patch", []string{"slide_id", "edits"}, map[string]any{"slide_id": slideIDSchema(pack), "edits": edits}),
	}
	if len(spec.FlattenOutline(pack.Outline.Outline)) > 0 || len(pack.Outline.Outline.Sections) > 0 {
		variants = append(variants[:1], variants[2:]...)
	}
	return map[string]any{"type": "object", "oneOf": variants, "description": "Closed discriminated union of the 12 supported PPT mutations."}
}
func slideIDSchema(pack contextengine.ContextPack) map[string]any {
	_ = pack
	return map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$"}
}

func readFailure(err error) ToolResult {
	if errors.Is(err, fs.ErrNotExist) {
		return failedToolResult(CodeResourceNotFound, "PPT resource was not found", false)
	}
	return failedToolResult("READ_FAILED", err.Error(), true)
}
func writeFailure(err error) ToolResult { return failedToolResult("WRITE_FAILED", err.Error(), true) }
func objectSchema(required []string, properties map[string]any) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
func stringValue(value any) string { text, _ := value.(string); return text }

var _ = spec.SchemaVersion
