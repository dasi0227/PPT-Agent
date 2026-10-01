package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/sourceformat"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

const maxPPTContentBytes = 2 * 1024 * 1024

type pptReadTool struct{ pack contextengine.ContextPack }

func (pptReadTool) Schema() ToolSchema {
	parameters := objectSchema([]string{"resource"}, map[string]any{"resource": resourceSchema(), "slide_id": map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$", "description": "Stable page ID from the current outline. Required for spec or html; omit for manifest, design and outline."}})
	parameters["if"] = map[string]any{"properties": map[string]any{"resource": map[string]any{"enum": []string{"spec", "html"}}}, "required": []string{"resource"}}
	parameters["then"] = map[string]any{"required": []string{"slide_id"}}
	parameters["else"] = map[string]any{"not": map[string]any{"required": []string{"slide_id"}}}
	return ToolSchema{Name: "read_resource", OutputSchema: toolOutputSchema("read_resource"), Description: "Read one resource. Spec and html require slide_id; global resources forbid it. Use the supplied outline context to determine whether initialization is needed. Availability of edit_outline does not imply an absent outline.", Parameters: parameters}
}
func (t pptReadTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	resource, err := parseResource(input.Args)
	if err != nil {
		return failedToolResult(CodeResourceInvalid, err.Error(), false)
	}
	if !AllowsRead(input.Scope, resource) {
		return failedToolResult(CodeTargetOutOfScope, "resource is outside scope", false)
	}
	ref, err := refForResource(t.pack, resource)
	if err != nil {
		return readFailure(err)
	}
	raw, _, err := readArtifact(input.ProjectDir, input.Session, ref)
	if err != nil {
		return resourceReadFailure(err, resource)
	}
	if len(raw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "resource exceeds read limit", false)
	}
	var content any = string(raw)
	if resource.Part != "html" {
		parsed, e := spec.ParseStrictSourceJSON(raw, resource.Part)
		if e != nil {
			return savedResourceInvalid(e)
		}
		if resource.Part != "outline" {
			content = parsed
		} else if e := spec.ValidateOutline(*parsed.(*spec.Outline)); e != nil {
			return savedResourceInvalid(e)
		}
	}
	data := map[string]any{"resource": resource.Part, "content": content}
	if resource.SlideID != "" {
		data["slide_id"] = resource.SlideID
	}
	out := SuccessfulToolResult("resource read")
	out.Data = data
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
func textEditsSchema() map[string]any {
	return map[string]any{"type": "array", "minItems": 1, "description": "Ordered exact text replacements on existing source. Each replacement sees the result of earlier replacements; the whole batch is saved atomically. Read the current source first.", "items": objectSchema([]string{"old_text", "new_text"}, map[string]any{
		"old_text": map[string]any{"type": "string", "minLength": 1, "description": "Exact non-empty source text to replace, including whitespace. It must occur exactly once when this replacement runs; this is not a regex or a diff."},
		"new_text": map[string]any{"type": "string", "description": "Literal replacement source text. Use an empty string to delete the matched text; the final document must remain valid."},
	})}
}
func (t resourceEditTool) Schema() ToolSchema {
	resource := resourceForTool(t.name, "")
	props := map[string]any{}
	required := []string{}
	description := "Edit supplied top-level fields; omitted fields remain unchanged, arrays replace completely."
	switch t.name {
	case "edit_manifest", "edit_design", "edit_spec":
		schemaName := pptschema.ManifestName
		if resource.Part == "design" {
			schemaName = pptschema.DesignName
		}
		if resource.Part == "spec" {
			schemaName = pptschema.SlideSpecName
		}
		schema := pptschema.AuthoringSchema(schemaName)
		props = schema["properties"].(map[string]any)
		if resource.Part == "design" {
			decorations := props["decorations"].(map[string]any)
			decorations["description"] = "Shared decoration positions to change. Only supplied keys are merged; omitted positions remain unchanged. Each non-none position may belong to only one configured decoration, even if its text is currently missing. The merged configuration is validated; conflicts are rejected. Move or hide the occupying decoration in the same edit when reassigning its position. left-edge and right-edge mean the vertical midpoint of that side. Text comes from presentation resources; Runtime and the theme control appearance."
			delete(decorations, "required")
			decorations["minProperties"] = 1
			description += " Decorations merge only the supplied position fields."
		}
		if resource.Part == "spec" {
			for _, key := range []string{"role", "layout"} {
				props[key] = map[string]any{"description": stringValue(props[key].(map[string]any)["description"]) + " Omit to keep the current value; pass null to remove it.", "anyOf": []any{props[key], map[string]any{"type": "null"}}}
			}
			description += " Requires slide_id. First creation requires key_message and elements. Set role or layout to null to remove that optional field."
		}
	case "edit_outline":
		props["init"] = map[string]any{"type": "object", "description": "Complete initial outline object with a sections array, used only when no outline exists; mutually exclusive with edits. sections lists top-level narrative groups in order; each section's subsections lists its second-level groups, and slides lists ordered page entries. title is the user-visible heading of the section, subsection or page; purpose states what a section or subsection contributes to the narrative. Keep unused slides/subsections arrays empty. " + outlineSourceContract}
		props["edits"] = textEditsSchema()
		props["edits"].(map[string]any)["description"] = "Ordered exact text replacements on existing outline JSON source, mutually exclusive with init. Read the saved source first; each replacement sees earlier replacements, and the batch must produce a valid outline. Section and subsection id values, and page slide_id values, are stable identities, not positions. " + outlineSourceContract
		description = "Initialize an absent outline with init, or edit existing source using edits. Supply exactly one. " + outlineSourceContract + " Runtime assigns new identities. Existing identities must be preserved. Removed pages delete their Spec and HTML atomically."
	case "edit_html":
		props["content"] = map[string]any{"type": "string", "minLength": 1, "description": "Complete HTML source for this existing outline page. Creates missing HTML or replaces all existing HTML; not a fragment, file path or Markdown code fence. Mutually exclusive with edits. Follow the slide HTML contract and render after saving."}
		props["edits"] = textEditsSchema()
		props["edits"].(map[string]any)["description"] = "Ordered exact text replacements on existing slide HTML, mutually exclusive with content. Read the current source first; each replacement sees earlier replacements, and the whole batch is saved atomically."
		description = "Create or replace slide HTML with content, or apply sequential exact replacements using edits. Supply exactly one. Each old_text must match once; all edits save atomically. HTML is formatted before saving; read_resource returns the exact saved source for subsequent replacements. Saving does not verify appearance; call render_slide."
	}
	if resource.Type == "slide" {
		props["slide_id"] = map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$", "description": "Stable ID of the existing outline page to edit, within the authorized page scope. Use its slide_id, not its title or page number; this tool does not create a page in the outline."}
		required = append(required, "slide_id")
	}
	parameters := objectSchema(required, props)
	if resource.Part == "outline" || resource.Part == "html" {
		key := "content"
		if resource.Part == "outline" {
			key = "init"
		}
		parameters["oneOf"] = []any{
			map[string]any{"required": []string{key}, "not": map[string]any{"required": []string{"edits"}}},
			map[string]any{"required": []string{"edits"}, "not": map[string]any{"required": []string{key}}},
		}
	} else {
		parameters["minProperties"] = len(required) + 1
	}
	return ToolSchema{Name: t.name, Description: description, Parameters: parameters, OutputSchema: toolOutputSchema(t.name)}
}
func (t resourceEditTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if input.Session == nil {
		return failedToolResult(CodeRunSessionRequired, "resource editing requires an active run session", false)
	}
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	argsRaw, _ := json.Marshal(input.Args)
	if len(argsRaw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "edit exceeds content limit", false)
	}
	resource := resourceForTool(t.name, stringValue(input.Args["slide_id"]))
	if !AllowsWrite(input.Scope, resource) {
		return failedToolResult(CodeTargetOutOfScope, "resource is outside scope", false)
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
	buffer := pptmutation.NewBuffer(runWorkspace{session: input.Session, pack: t.pack, source: t.name})
	engine := pptmutation.Service{Workspace: buffer, FormatHTML: func(raw []byte) ([]byte, error) { return sourceformat.HTML(ctx, raw) }, ValidateHTML: func(raw []byte) error { _, err := validateHTML(raw); return err }}
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
		edited, err = engine.EditResource(pptmutation.ResourceEdit{Resource: resource.Part, SlideID: resource.SlideID, ExpectedHash: expectedHash, Fields: fields, Content: string(initialRaw), Edits: edits, Initialize: initialize})
		raw, changedFields = edited.Content, edited.ChangedFields
	}
	if err != nil {
		return resourceMutationFailure(err, resource)
	}
	if len(raw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "result exceeds content limit", false)
	}
	changed := buffer.HasChanges()
	if err = buffer.Commit(); err != nil {
		return writeFailure(err)
	}
	ref, _ := refForResource(t.pack, resource)
	raw, _, err = readArtifact(input.ProjectDir, input.Session, ref)
	if err != nil {
		return readFailure(err)
	}
	out := SuccessfulToolResult("resource saved")
	out.Data = map[string]any{}
	if resource.Part == "html" {
		out.Summary = "HTML 已保存。"
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
