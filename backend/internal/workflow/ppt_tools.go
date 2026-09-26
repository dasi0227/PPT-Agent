package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

const maxPPTContentBytes = 2 * 1024 * 1024

type pptReadTool struct{ pack contextengine.ContextPack }

func (pptReadTool) Schema() ToolSchema {
	return ToolSchema{Name: "read_resource", Description: "Read one resource. Manifest, design and spec return complete JSON objects; outline and html return exact saved source text. Spec and html require slide_id; global resources forbid it.", Parameters: objectSchema([]string{"resource"}, map[string]any{"resource": resourceSchema(), "slide_id": map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$"}})}
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
		if resource.Part == "outline" && errors.Is(err, fs.ErrNotExist) {
			return failedToolResult(CodeResourceNotFound, "outline is not initialized; use init_outline", false)
		}
		return readFailure(err)
	}
	if len(raw) > maxPPTContentBytes {
		return failedToolResult(CodeContentTooLarge, "resource exceeds read limit", false)
	}
	var content any = string(raw)
	if resource.Part != "html" {
		parsed, e := spec.ParseStrictSourceJSON(raw, resource.Part)
		if e != nil {
			return readFailure(e)
		}
		if resource.Part != "outline" {
			content = parsed
		} else if e := spec.ValidateOutline(*parsed.(*spec.Outline)); e != nil {
			return readFailure(e)
		}
	}
	data := map[string]any{"ok": true, "resource": resource.Part, "content": content, "content_hash": resourceContentHash(resource, raw)}
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
	observation, _ := json.Marshal(out.Data)
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
	case "init_outline", "arrange_outline":
		return Resource{Type: "deck", Part: "outline"}
	case "edit_spec":
		return Resource{Type: "slide", SlideID: slideID, Part: "spec"}
	case "write_html", "patch_html":
		return Resource{Type: "slide", SlideID: slideID, Part: "html"}
	}
	return Resource{}
}
func isResourceEditTool(name string) bool { return resourceForTool(name, "").Type != "" }
func textEditsSchema() map[string]any {
	return map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"old_text", "new_text"}, map[string]any{"old_text": map[string]any{"type": "string", "minLength": 1}, "new_text": map[string]any{"type": "string"}})}
}
func (t resourceEditTool) Schema() ToolSchema {
	resource := resourceForTool(t.name, "")
	props := map[string]any{}
	required := []string{}
	description := "Edit supplied top-level fields; omitted fields remain unchanged, arrays replace completely. Returns the complete saved object."
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
			delete(decorations, "required")
			decorations["minProperties"] = 1
			description += " Decorations merge only the supplied position fields."
		}
		if resource.Part == "spec" {
			for _, key := range []string{"role", "layout"} {
				props[key] = map[string]any{"anyOf": []any{props[key], map[string]any{"type": "null"}}}
			}
			description += " Requires slide_id. First creation requires key_message and elements. Set role or layout to null to remove that optional field."
		}
	case "init_outline":
		props["content"] = map[string]any{"type": "string", "minLength": 1}
		required = append(required, "content")
		description = "Initialize the absent outline from complete JSON source. Omit IDs for every new node; Runtime assigns them. Sections require title, purpose, slides and subsections arrays; subsections require title, purpose and slides. Returns the exact saved JSON source with IDs. Never overwrites an existing outline."
	case "arrange_outline":
		props["edits"] = textEditsSchema()
		required = append(required, "edits")
		description = "Edit existing outline JSON source with sequential exact replacements. Each old_text must match once. Preserve existing IDs; omit IDs for new nodes. Removed page IDs delete their Spec and HTML. Final structure and related changes commit atomically. Returns the exact saved source."
	case "write_html":
		props["html"] = map[string]any{"type": "string", "minLength": 1}
		required = append(required, "html")
		description = "Create or fully replace one existing slide's HTML source. Returns changed and content_hash; render separately to inspect appearance."
	case "patch_html":
		props["edits"] = textEditsSchema()
		required = append(required, "edits")
		description = "Apply sequential exact text replacements to existing slide HTML. Each old_text must match once; all edits commit atomically. Returns changed and content_hash."
	}
	if resource.Type == "slide" {
		props["slide_id"] = map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$"}
		required = append(required, "slide_id")
	}
	if t.name != "init_outline" {
		props["expected_hash"] = map[string]any{"type": "string", "pattern": "^sha256:[a-f0-9]{64}$", "description": "Version from read_resource or a previous edit. Supply it when editing existing content; on conflict read again."}
	}
	return ToolSchema{Name: t.name, Description: description, Parameters: objectSchema(required, props)}
}
func (t resourceEditTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	if input.Session == nil {
		return failedToolResult(CodeRunSessionRequired, "resource editing requires an active run session", false)
	}
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return failedToolResult(CodeContentInvalid, err.Error(), false)
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
		if k != "slide_id" && k != "expected_hash" {
			fields[k] = v
		}
	}
	edits := []pptmutation.Edit{}
	if value, ok := input.Args["edits"]; ok {
		raw, _ := json.Marshal(value)
		_ = json.Unmarshal(raw, &edits)
	}
	buffer := pptmutation.NewBuffer(runWorkspace{session: input.Session, pack: t.pack, source: t.name})
	engine := pptmutation.Service{Workspace: buffer, ValidateHTML: func(raw []byte) error { _, err := validateHTML(raw); return err }}
	var raw []byte
	changedFields := []string{}
	var err error
	if resource.Part == "html" {
		op := "slide.html.write"
		if t.name == "patch_html" {
			op = "slide.html.patch"
		}
		_, err = engine.Apply(pptmutation.Request{Op: op, SlideID: resource.SlideID, HTML: stringValue(input.Args["html"]), Edits: edits, ExpectedHash: stringValue(input.Args["expected_hash"])})
		if err == nil {
			raw, err = buffer.Read(slideHTMLRef(resource.SlideID).Path)
		}
	} else {
		var edited pptmutation.ResourceEditResult
		edited, err = engine.EditResource(pptmutation.ResourceEdit{Resource: resource.Part, SlideID: resource.SlideID, ExpectedHash: stringValue(input.Args["expected_hash"]), Fields: fields, Content: stringValue(input.Args["content"]), Edits: edits, Initialize: t.name == "init_outline"})
		raw, changedFields = edited.Content, edited.ChangedFields
	}
	if err != nil {
		code := CodeContentInvalid
		if errors.Is(err, pptmutation.ErrContentConflict) {
			code = CodeContentConflict
		}
		return failedToolResult(code, err.Error(), true)
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
	out.Data = map[string]any{"ok": true, "changed": changed, "content_hash": resourceContentHash(resource, raw)}
	if resource.SlideID != "" {
		out.Data["slide_id"] = resource.SlideID
	}
	if resource.Part == "outline" {
		out.Data["content"] = string(raw)
	} else if resource.Part != "html" {
		var value any
		_ = json.Unmarshal(raw, &value)
		out.Data[resource.Part] = value
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
	if resource.Part != "html" {
		setResourceObservation(&out, resource, raw)
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

// Re-evaluate before every model request, after the previous transaction commits.
func discloseOutlineState(schemas []ToolSchema, projectDir string, session *RunSession) []ToolSchema {
	var err error
	if session != nil {
		_, err = session.ReadPath(".outline.json")
	} else {
		_, err = os.Stat(filepath.Join(projectDir, ".outline.json"))
	}
	missing := errors.Is(err, fs.ErrNotExist)
	out := make([]ToolSchema, 0, len(schemas))
	for _, schema := range schemas {
		if schema.Name == "init_outline" && !missing || schema.Name == "arrange_outline" && missing {
			continue
		}
		out = append(out, schema)
	}
	return out
}
