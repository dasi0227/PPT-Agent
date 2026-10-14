package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

type pptReadTool struct{}

func (pptReadTool) Schema() ToolSchema {
	return tooldefinition.Get("read_resource")
}
func (t pptReadTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	resource, err := parseResource(input.Args)
	if err != nil {
		return failedToolResult(CodeToolArgumentInvalid, err.Error())
	}
	if resource.Type == "slide" {
		if _, err := readPageOutline(input.Context, input.ProjectDir, input.Session, resource.SlideID); err != nil {
			return resourceReadToolFailure(err, resource)
		}
	}
	ref, err := refForResource(input.Context, resource)
	if err != nil {
		return readFailure(err)
	}
	raw, _, err := readArtifact(input.ProjectDir, input.Session, ref)
	if err != nil {
		return resourceReadToolFailure(err, resource)
	}
	if resource.Part != "html" {
		if _, err := spec.ParseStrictSourceJSON(raw, resource.Part); err != nil {
			return savedResourceInvalid(err, resource)
		}
	}
	out := SuccessfulToolResult("resource read")
	out.Data = map[string]any{"content": string(raw)}
	setResourceObservation(&out, resource, raw)
	// Exact source reads also authorize later text edits of this physical file.
	out.ObservationMetadata = commandVersionMetadata(ref.Path, raw)
	return out
}

func resourceContentHash(resource Resource, raw []byte) string {
	if resource.Part == "html" || resource.Part == "outline" {
		return spec.ContentHash(raw)
	}
	return spec.ResourceBytesHash(raw)
}
func setResourceObservation(out *ToolResult, resource Resource, raw []byte) {
	out.Observation = modelToolObservation(*out)
	out.ObservationMetadata = &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: []llm.ResourceStamp{{Key: "ppt/" + resource.Key(), Hash: resourceContentHash(resource, raw)}}}
}

type resourceEditTool struct {
	name string
}

func resourceForTool(name, slideID string) Resource {
	switch name {
	case "edit_manifest":
		return Resource{Type: "deck", Part: "manifest"}
	case "edit_design":
		return Resource{Type: "deck", Part: "design"}
	case "edit_outline":
		return Resource{Type: "deck", Part: "outline"}
	case "edit_spec":
		return Resource{Type: "slide", SlideID: slideID, Part: "spec"}
	case "edit_html":
		return Resource{Type: "slide", SlideID: slideID, Part: "html"}
	}
	return Resource{}
}
func isResourceEditTool(name string) bool { return resourceForTool(name, "").Type != "" }

func isFieldEditTool(name string) bool {
	return name == "edit_manifest" || name == "edit_design" || name == "edit_spec"
}

func (t resourceEditTool) Schema() ToolSchema {
	return tooldefinition.Get(t.name)
}
func (t resourceEditTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if input.Session == nil {
		return failedToolResult(model.CodeInternal, "resource editing requires an active run session")
	}
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	resource := resourceForTool(t.name, stringValue(input.Args["slide_id"]))
	if !AllowsWrite(input.Scope, resource) {
		return failedToolResult(CodeTargetOutOfScope, "resource is outside scope")
	}
	if resource.Type == "slide" {
		if _, err := readPageOutline(input.Context, input.ProjectDir, input.Session, resource.SlideID); err != nil {
			return resourceMutationFailure(err, resource)
		}
		input.Session.requireSlide(resource.SlideID)
	}
	fields := map[string]any{}
	for k, v := range input.Args {
		if k != "slide_id" {
			fields[k] = v
		}
	}
	edits := []pptmutation.Edit{}
	if value, ok := input.Args["edits"]; ok {
		raw, _ := json.Marshal(value)
		_ = json.Unmarshal(raw, &edits)
	}
	initial, initialize := input.Args["init"]
	expectedHash, versionErr := "", error(nil)
	if !initialize {
		expectedHash, versionErr = modelSeenResourceVersion(input, resource)
	}
	if versionErr != nil {
		return resourceMutationFailure(versionErr, resource)
	}
	initialRaw, _ := json.Marshal(initial)
	buffer := pptmutation.NewBuffer(runWorkspace{session: input.Session, pack: input.Context, source: t.name})
	engine := pptmutation.Service{Workspace: buffer, ValidateHTML: func(raw []byte) error { _, err := validateHTML(raw); return err }}
	var err error
	if resource.Part == "html" {
		op := "slide.html.write"
		if _, patch := input.Args["edits"]; patch {
			op = "slide.html.patch"
		}
		_, err = engine.Apply(pptmutation.Request{Op: op, SlideID: resource.SlideID, HTML: stringValue(input.Args["content"]), Edits: edits, ExpectedHash: expectedHash})
	} else {
		err = engine.EditResource(pptmutation.ResourceEdit{Resource: resource.Part, ProjectTitle: input.Context.Project.Title, SlideID: resource.SlideID, ExpectedHash: expectedHash, Fields: fields, Content: string(initialRaw), Edits: edits, Initialize: initialize})
	}
	if err != nil {
		return resourceMutationFailure(err, resource)
	}
	changed := buffer.HasChanges()
	if err = buffer.Commit(); err != nil {
		return internalToolFailure(err)
	}
	ref, _ := refForResource(input.Context, resource)
	raw, _, err := readArtifact(input.ProjectDir, input.Session, ref)
	if err != nil {
		return internalToolFailure(err)
	}
	input.Session.recordUnchangedResource(resource, ref, raw)
	out := SuccessfulToolResult("resource saved")
	if resource.Part == "html" {
		out.Summary = "HTML 已保存。"
		if !changed {
			out.Summary = "内容未变化；本次编辑未修复任何问题。"
		}
	} else if resource.Part == "outline" {
		out.Data = map[string]any{"content": string(raw)}
	} else {
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return internalToolFailure(err)
		}
		out.Data = map[string]any{"content": value}
	}
	if changed {
		out.ChangedTargets = []ChangedTarget{{Type: resource.Type, Part: resource.Part, SlideID: resource.SlideID, Hash: hashBytes(raw)}}
		if resource.Part == "html" {
			out.InvalidatedTargets = []Resource{resource}
		}
	}
	kind := "schema"
	if resource.Part == "html" {
		kind = "static"
	}
	out.Evidence = []Evidence{newEvidence(kind, resource, hashBytes(raw), map[string]any{"valid": true})}
	setResourceObservation(&out, resource, raw)
	return out
}

type runWorkspace struct {
	session *RunSession
	pack    contextengine.ContextPack
	source  string
}

func (w runWorkspace) Read(path string) ([]byte, error) {
	return w.session.Read(refForPath(w.pack, path))
}
func (w runWorkspace) Write(path string, raw []byte) error {
	_, err := w.session.Write(refForPath(w.pack, path), w.source, raw)
	return err
}
func (w runWorkspace) Delete(path string) error {
	return w.session.Delete(refForPath(w.pack, path), w.source)
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
	if id, ok := model.SlideIDFromSpecPath(path); ok {
		return specSlideRef(id)
	}
	if id, ok := model.SlideIDFromHTMLPath(path); ok {
		return slideHTMLRef(id)
	}
	return ArtifactRef{Kind: ArtifactDerived, ID: path, Path: path, Project: pack.Project.ID}
}

func readFailure(err error) ToolResult {
	if errors.Is(err, fs.ErrNotExist) {
		return failedToolResult(model.CodeNotFound, "PPT resource was not found: "+err.Error())
	}
	return internalToolFailure(err)
}
func stringValue(value any) string { text, _ := value.(string); return text }
