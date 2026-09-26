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
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
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
	hash := spec.ResourceBytesHash(raw)
	if resource.Part == "html" {
		hash = spec.ContentHash(raw)
	}
	result.Data = map[string]any{"hash": hash}
	var content any = string(raw)
	if resource.Part == "outline" {
		var outline spec.Outline
		if err = json.Unmarshal(raw, &outline); err != nil {
			return readFailure(err)
		}
		content = contextengine.ModelOutline(contextengine.ContextPack{Outline: contextengine.OutlineContext{Outline: outline}})
	} else if resource.Part != "html" {
		content, err = contextengine.ModelResource(raw)
		if err != nil {
			return readFailure(err)
		}
	}
	stamp := llm.ResourceStamp{Key: "ppt/" + resource.Key(), Hash: hash}
	target := map[string]any{"resource": resource, "content_hash": hash}
	if visibleResourceHashes(input.Messages)[stamp.Key] == stamp.Hash {
		target["already_available"] = true
	} else {
		target["content"] = content
		target["replaces_previous"] = true
		result.ObservationMetadata = &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: []llm.ResourceStamp{stamp}}
	}
	if resource.SlideID != "" {
		target["display_name"] = runtimeSlideDisplayName(input.ProjectDir, resource.SlideID)
	}
	observation, _ := json.Marshal(target)
	result.Observation = string(observation)
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
	engine := pptmutation.Service{Workspace: buffer, ValidateHTML: func(raw []byte) error {
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
		case errors.Is(err, pptmutation.ErrContentConflict):
			code = CodeContentConflict
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
	out.Data = map[string]any{"operation": result.Operation, "hashes": result.Hashes, "created": result.Created, "affected_slide_ids": result.AffectedSlideIDs}
	if !buffer.HasChanges() {
		out.Summary = "PPT content unchanged"
		return out
	}
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
			out.ChangedTargets = []ChangedTarget{{Type: resource.Type, SlideID: resource.SlideID, Part: resource.Part, Hash: hash}}
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
	case ".manifest.json":
		return manifestRef(pack)
	case ".outline.json":
		return outlineRef(pack)
	case ".design.json":
		return designRef(pack)
	}
	if strings.HasSuffix(path, ".html") && stableSlideID.MatchString(strings.TrimSuffix(path, ".html")) {
		return slideHTMLRef(strings.TrimSuffix(path, ".html"))
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
	definitions := map[string]any{
		"manifest": pptschema.AuthoringSchema(pptschema.ManifestName),
		"design":   pptschema.AuthoringSchema(pptschema.DesignName),
		"spec":     pptschema.AuthoringSchema(pptschema.SlideSpecName),
	}
	patch := func(operation string) map[string]any {
		name := strings.Split(operation, ".")[0]
		if name == "slide" {
			name = "spec"
		}
		return mutationPatchSchema(operation, name, definitions[name].(map[string]any))
	}
	position := objectSchema(nil, map[string]any{"parent_id": map[string]any{"type": "string"}, "before_id": map[string]any{"type": "string"}, "after_id": map[string]any{"type": "string"}})
	variant := func(op string, required []string, props map[string]any) any {
		props["op"] = map[string]any{"const": op}
		props["expected_hash"] = map[string]any{"type": "string", "pattern": "^sha256:[a-f0-9]{64}$"}
		return objectSchema(append([]string{"op"}, required...), props)
	}
	text := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	draftSlide := pptschema.OutlineDraftSchema("slide")
	draftSubsection := pptschema.OutlineDraftSchema("subsection")
	draftSection := pptschema.OutlineDraftSchema("section")
	withKind := func(kind string, node map[string]any) map[string]any {
		props := node["properties"].(map[string]any)
		props["kind"] = map[string]any{"const": kind}
		node["required"] = append(node["required"].([]string), "kind")
		return node
	}
	// Inserting a subsection creates an empty parent; slides are separate inserts.
	subProps := draftSubsection["properties"].(map[string]any)
	delete(subProps, "slides")
	draftSubsection["required"] = []string{"client_ref", "title", "purpose"}
	draftNode := map[string]any{"oneOf": []any{withKind("section", pptschema.OutlineDraftSchema("section")), withKind("subsection", draftSubsection), withKind("slide", draftSlide)}}
	slideProps := pptschema.OutlineDraftSchema("slide")["properties"].(map[string]any)
	sectionProps := draftSection["properties"].(map[string]any)
	changes := objectSchema(nil, map[string]any{"title": slideProps["title"], "purpose": sectionProps["purpose"]})
	changes["minProperties"] = 1
	design := map[string]any{"$ref": "#/$defs/design"}
	slideSpec := map[string]any{"$ref": "#/$defs/spec"}
	edits := map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"old_text", "new_text"}, map[string]any{"old_text": text(maxPPTContentBytes), "new_text": map[string]any{"type": "string", "maxLength": maxPPTContentBytes}})}
	outlineInit := variant("outline.init", []string{"structure"}, map[string]any{"structure": map[string]any{"type": "array", "minItems": 1, "maxItems": pptschema.AuthoringSchema(pptschema.OutlineName)["properties"].(map[string]any)["sections"].(map[string]any)["maxItems"], "items": draftSection}}).(map[string]any)
	outlineInit["examples"] = []any{map[string]any{
		"op": "outline.init",
		"structure": []any{map[string]any{
			"client_ref": "opening", "title": "Opening", "purpose": "Introduce the topic",
			"slides":      []any{map[string]any{"client_ref": "cover", "title": "Presentation title"}},
			"subsections": []any{},
		}},
	}, map[string]any{
		"op": "outline.init",
		"structure": []any{map[string]any{
			"client_ref": "analysis", "title": "Analysis", "purpose": "Explain the key findings",
			"slides": []any{},
			"subsections": []any{map[string]any{
				"client_ref": "market", "title": "Market context", "purpose": "Establish the external context",
				"slides": []any{map[string]any{"client_ref": "market_shift", "title": "The market is shifting"}},
			}},
		}},
	}}
	variants := []any{
		variant("manifest.patch", []string{"patch"}, map[string]any{"patch": patch("manifest.patch")}),
		outlineInit,
		variant("outline.insert", []string{"node", "position"}, map[string]any{"node": draftNode, "position": position, "direct_slides_policy": map[string]any{"enum": []string{"move_into_new_subsection"}}}),
		variant("outline.move", []string{"node_id", "position"}, map[string]any{"node_id": map[string]any{"type": "string"}, "position": position}),
		variant("outline.update", []string{"node_id", "changes"}, map[string]any{"node_id": map[string]any{"type": "string"}, "changes": changes}),
		variant("outline.remove", []string{"node_id"}, map[string]any{"node_id": map[string]any{"type": "string"}, "child_policy": map[string]any{"enum": []string{"promote_to_section"}}}),
		variant("design.write", []string{"design"}, map[string]any{"design": design}), variant("design.patch", []string{"patch"}, map[string]any{"patch": patch("design.patch")}),
		variant("slide.spec.write", []string{"slide_id", "spec"}, map[string]any{"slide_id": slideIDSchema(pack), "spec": slideSpec}), variant("slide.spec.patch", []string{"slide_id", "patch"}, map[string]any{"slide_id": slideIDSchema(pack), "patch": patch("slide.spec.patch")}),
		variant("slide.html.write", []string{"slide_id", "html"}, map[string]any{"slide_id": slideIDSchema(pack), "html": map[string]any{"type": "string", "minLength": 1, "maxLength": maxPPTContentBytes}}), variant("slide.html.patch", []string{"slide_id", "edits"}, map[string]any{"slide_id": slideIDSchema(pack), "edits": edits}),
	}
	return map[string]any{"type": "object", "oneOf": variants, "$defs": definitions, "description": "Closed discriminated union of the 12 supported PPT mutations."}
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
