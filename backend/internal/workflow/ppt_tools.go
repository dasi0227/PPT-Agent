package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

const maxPPTContentBytes = 2 * 1024 * 1024

type pptReadTool struct{ pack contextengine.ContextPack }

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
		outlineResource := Resource{Type: "deck", Part: "outline"}
		outlineRaw, _, err := readArtifact(input.ProjectDir, input.Session, outlineRef(input.Context))
		if err != nil {
			return resourceReadToolFailure(err, outlineResource)
		}
		parsed, err := spec.ParseStrictSourceJSON(outlineRaw, "outline")
		if err != nil {
			return resourceReadInvalid(err, outlineResource)
		}
		if _, exists := spec.FindSlide(*parsed.(*spec.Outline), resource.SlideID); !exists {
			return resourceReadToolFailure(pptmutation.ErrSlideNotFound, resource)
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
	var content any = string(raw)
	if resource.Part != "html" {
		parsed, e := spec.ParseStrictSourceJSON(raw, resource.Part)
		if e != nil {
			return resourceReadInvalid(e, resource)
		}
		if resource.Part != "outline" {
			content = parsed
		}
	}
	out := SuccessfulToolResult("resource read")
	out.Data = map[string]any{"content": content}
	setResourceObservation(&out, resource, raw)
	return out
}

func resourceContentHash(resource Resource, raw []byte) string {
	if resource.Part == "html" || resource.Part == "outline" {
		return spec.ContentHash(raw)
	}
	return spec.ResourceBytesHash(raw)
}
func setResourceObservation(out *ToolResult, resource Resource, raw []byte) {
	observation, _ := json.Marshal(contextengine.ModelValue(out.Data))
	out.Observation = string(observation)
	out.ObservationMetadata = &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: []llm.ResourceStamp{{Key: "ppt/" + resource.Key(), Hash: resourceContentHash(resource, raw)}}}
}

type resourceEditTool struct {
	pack contextengine.ContextPack
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
	argsRaw, _ := json.Marshal(input.Args)
	if len(argsRaw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "edit exceeds content limit")
	}
	resource := resourceForTool(t.name, stringValue(input.Args["slide_id"]))
	if !AllowsWrite(input.Scope, resource) {
		return failedToolResult(CodeTargetOutOfScope, "resource is outside scope")
	}
	if resource.Type == "slide" {
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
	var raw []byte
	changedFields := []string{}
	var err error
	if resource.Part == "html" {
		op := "slide.html.write"
		if _, patch := input.Args["edits"]; patch {
			op = "slide.html.patch"
		}
		_, err = engine.Apply(pptmutation.Request{Op: op, SlideID: resource.SlideID, HTML: stringValue(input.Args["content"]), Edits: edits, ExpectedHash: expectedHash})
		if err == nil {
			raw, err = buffer.Read(slideHTMLRef(resource.SlideID).Path)
		}
	} else {
		var edited pptmutation.ResourceEditResult
		edited, err = engine.EditResource(pptmutation.ResourceEdit{Resource: resource.Part, ProjectTitle: input.Context.Project.Title, SlideID: resource.SlideID, ExpectedHash: expectedHash, Fields: fields, Content: string(initialRaw), Edits: edits, Initialize: initialize})
		raw, changedFields = edited.Content, edited.ChangedFields
	}
	if err != nil {
		return resourceMutationFailure(err, resource)
	}
	if len(raw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "result exceeds content limit")
	}
	changed := buffer.HasChanges()
	if err = buffer.Commit(); err != nil {
		return internalToolFailure(err)
	}
	ref, _ := refForResource(input.Context, resource)
	raw, _, err = readArtifact(input.ProjectDir, input.Session, ref)
	if err != nil {
		return internalToolFailure(err)
	}
	input.Session.recordUnchangedResource(resource, ref, raw)
	out := SuccessfulToolResult("resource saved")
	out.Data = map[string]any{"changed": changed}
	if resource.Part == "html" {
		out.Summary = "HTML 已保存。"
		if !changed {
			out.Summary = "内容未变化；本次编辑未修复任何问题。"
		}
		out.Data["summary"] = out.Summary
	} else if resource.Part == "outline" {
		out.Data["content"] = string(raw)
	} else {
		var value any
		_ = json.Unmarshal(raw, &value)
		out.Data["content"] = value
		out.Data["changed_fields"] = changedFields
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
	if resource.Part == "html" {
		out.Observation = modelToolObservation(out)
	}
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
	if strings.HasSuffix(path, ".html") && stableSlideID.MatchString(strings.TrimSuffix(path, ".html")) {
		return slideHTMLRef(strings.TrimSuffix(path, ".html"))
	}
	return ArtifactRef{Kind: ArtifactDerived, ID: path, Path: path, Project: pack.Project.ID}
}

func readFailure(err error) ToolResult {
	if errors.Is(err, fs.ErrNotExist) {
		return failedToolResult(CodeResourceNotFound, "PPT resource was not found")
	}
	return internalToolFailure(err)
}
func stringValue(value any) string { text, _ := value.(string); return text }
