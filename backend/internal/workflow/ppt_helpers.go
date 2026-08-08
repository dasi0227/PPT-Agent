package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	nethtml "golang.org/x/net/html"
)

var stableSlideID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

const resourceObjectGuidance = `resource must be an object, not a string or path. Use {"type":"deck","part":"outline"}, {"type":"deck","part":"design"}, or {"type":"slide","slide_id":"<stable slide_id>","part":"spec|html"}. Never use display keys such as "deck:outline" or "slide:<id>:html", "current", or "slides/...".`

func outlineRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactOutline, ID: pack.Project.ID, Path: "outline.json", Project: pack.Project.ID}
}

func designRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactDesign, ID: pack.Project.ID, Path: "design.json", Project: pack.Project.ID}
}

func designTokensRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{
		Kind: ArtifactDerived, ID: pack.Project.ID + ":design-tokens",
		Path: "common/tokens.css", Project: pack.Project.ID,
	}
}

func specSlideRef(id string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactSlideSpec, ID: id, Path: model.SlideSpecPath(id)}
}

func slideHTMLRef(id string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactSlideHTML, ID: id, Path: model.SlideHTMLPath(id)}
}

func refForResource(pack contextengine.ContextPack, resource Resource) (ArtifactRef, error) {
	switch resource.Key() {
	case "deck:outline":
		return outlineRef(pack), nil
	case "deck:design":
		return designRef(pack), nil
	}
	if resource.Type == "slide" && resource.Part == "spec" {
		return specSlideRef(resource.SlideID), nil
	}
	if resource.Type == "slide" && resource.Part == "html" {
		return slideHTMLRef(resource.SlideID), nil
	}
	return ArtifactRef{}, fmt.Errorf("unsupported PPT resource")
}

func parseResource(args map[string]any) (Resource, error) {
	raw, exists := args["resource"]
	if !exists {
		return Resource{}, fmt.Errorf(resourceObjectGuidance)
	}
	if _, ok := raw.(string); ok {
		return Resource{}, fmt.Errorf(resourceObjectGuidance)
	}
	value, ok := raw.(map[string]any)
	if !ok {
		return Resource{}, fmt.Errorf(resourceObjectGuidance)
	}
	resource := Resource{
		Type: stringValue(value["type"]), SlideID: stringValue(value["slide_id"]),
		Part: stringValue(value["part"]),
	}
	if resource.Type == "deck" {
		if resource.SlideID != "" || (resource.Part != "outline" && resource.Part != "design") {
			return Resource{}, fmt.Errorf(`deck resource requires {"type":"deck","part":"outline|design"} and forbids slide_id`)
		}
		return resource, nil
	}
	if resource.Type == "slide" {
		if !stableSlideID.MatchString(resource.SlideID) || resource.SlideID == "current" {
			return Resource{}, fmt.Errorf(`slide resource requires a stable slide_id from the current project; never use "current" or a path such as "slides/..."`)
		}
		if resource.Part != "spec" && resource.Part != "html" {
			return Resource{}, fmt.Errorf(`slide resource requires {"type":"slide","slide_id":"<stable slide_id>","part":"spec|html"}`)
		}
		return resource, nil
	}
	return Resource{}, fmt.Errorf(`resource type must be "deck" or "slide"; ` + resourceObjectGuidance)
}

func readArtifact(projectDir string, tx *RunSession, ref ArtifactRef) ([]byte, string, error) {
	if tx != nil {
		raw, err := tx.Read(ref)
		if err == nil {
			source := "committed"
			if tx.HasChange(ref) {
				source = "direct_write"
			}
			return raw, source, nil
		}
		return nil, "", err
	}
	raw, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(ref.Path)))
	return raw, "committed", err
}

func errorsIsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

func readFailure(err error) ToolResult {
	if errorsIsNotExist(err) {
		return failedToolResult(CodeResourceNotFound, "PPT resource was not found", false)
	}
	return failedToolResult("READ_FAILED", err.Error(), true)
}

func writeFailure(err error) ToolResult {
	if errors.Is(err, ErrArtifactHashMismatch) {
		return failedToolResult(CodeRevisionConflict, err.Error(), true)
	}
	return failedToolResult("WRITE_FAILED", err.Error(), true)
}

func resourceSchema() map[string]any {
	return map[string]any{
		"description": resourceObjectGuidance,
		"oneOf": []any{
			objectSchema([]string{"type", "part"}, map[string]any{
				"type": map[string]any{"const": "deck", "description": `Use "deck" for deck-wide resources.`},
				"part": map[string]any{
					"type": "string", "enum": []string{"outline", "design"},
					"description": `Use "outline" for deck narrative/order, or "design" for deck-wide visual system.`,
				},
			}),
			objectSchema([]string{"type", "slide_id", "part"}, map[string]any{
				"type": map[string]any{"const": "slide", "description": `Use "slide" for one page resource.`},
				"slide_id": map[string]any{
					"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`,
					"description": `Stable slide identifier from the current project, for example "slide-01"; never use "current" or a file path.`,
				},
				"part": map[string]any{
					"type": "string", "enum": []string{"spec", "html"},
					"description": `Use "spec" for the page design/spec JSON, or "html" for the final slide implementation.`,
				},
			}),
		},
	}
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	return map[string]any{
		"type": "object", "required": required, "properties": properties, "additionalProperties": false,
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func intValue(value any, fallback int) int {
	switch current := value.(type) {
	case float64:
		return int(current)
	case int:
		return current
	case string:
		if parsed, err := strconv.Atoi(current); err == nil {
			return parsed
		}
	}
	return fallback
}

func normalizeModel(pack contextengine.ContextPack, tx *RunSession, ref ArtifactRef, value any) ([]byte, int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, 0, err
	}
	now := time.Now().Unix()
	switch ref.Kind {
	case ArtifactOutline:
		var next spec.Outline
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, 0, err
		}
		var current spec.Outline
		currentRaw, _, readErr := readArtifact(tx.ProjectDir(), tx, ref)
		if readErr == nil {
			_ = json.Unmarshal(currentRaw, &current)
		}
		next.SchemaVersion = spec.SchemaVersion
		next.ProjectID = pack.Project.ID
		next.Revision = maxInt(current.Revision+1, 1)
		if current.CreatedAt != 0 {
			next.CreatedAt = current.CreatedAt
		} else {
			next.CreatedAt = now
		}
		next.UpdatedAt = now
		if err := validateOutlineStructure(next); err != nil {
			return nil, 0, err
		}
		raw, _ = json.MarshalIndent(next, "", "  ")
		return raw, next.Revision, nil
	case ArtifactSlideSpec:
		var next spec.SlideSpec
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, 0, err
		}
		var current spec.SlideSpec
		currentRaw, _, readErr := readArtifact(tx.ProjectDir(), tx, ref)
		if readErr == nil {
			_ = json.Unmarshal(currentRaw, &current)
		}
		next.SchemaVersion = spec.SchemaVersion
		next.ProjectID = pack.Project.ID
		next.SlideID = ref.ID
		if outline, outlineErr := currentOutline(pack, tx); outlineErr == nil {
			next.SourceOutlineRevision = outline.Revision
		}
		next.Revision = maxInt(current.Revision+1, 1)
		if current.CreatedAt != 0 {
			next.CreatedAt = current.CreatedAt
		} else {
			next.CreatedAt = now
		}
		next.UpdatedAt = now
		if err := spec.ValidateSlideSpec(next); err != nil {
			return nil, 0, err
		}
		raw, _ = json.MarshalIndent(next, "", "  ")
		return raw, next.Revision, nil
	case ArtifactDesign:
		var next spec.Design
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, 0, err
		}
		var current spec.Design
		currentRaw, _, readErr := readArtifact(tx.ProjectDir(), tx, ref)
		if readErr == nil {
			_ = json.Unmarshal(currentRaw, &current)
		}
		next.SchemaVersion = spec.SchemaVersion
		next.ProjectID = pack.Project.ID
		next.Revision = maxInt(current.Revision+1, 1)
		if current.CreatedAt != 0 {
			next.CreatedAt = current.CreatedAt
		} else {
			next.CreatedAt = now
		}
		next.UpdatedAt = now
		if err := spec.ValidateDesign(next); err != nil {
			return nil, 0, err
		}
		if strings.TrimSpace(next.Theme) == "" || strings.TrimSpace(next.Direction) == "" ||
			strings.TrimSpace(next.Density) == "" || next.Chrome == nil {
			return nil, 0, fmt.Errorf("%w: incomplete design spec", spec.ErrInvalid)
		}
		raw, _ = json.MarshalIndent(next, "", "  ")
		return raw, next.Revision, nil
	default:
		return nil, 0, errors.New("target does not accept a JSON model")
	}
}

func validateOutlineStructure(deck spec.Outline) error {
	if deck.SchemaVersion != spec.SchemaVersion || deck.Revision < 1 ||
		deck.ProjectID == "" || strings.TrimSpace(deck.Title) == "" ||
		strings.TrimSpace(deck.Goal) == "" || strings.TrimSpace(deck.Audience) == "" ||
		strings.TrimSpace(deck.Language) == "" || deck.Sections == nil || deck.SlideOrder == nil {
		return fmt.Errorf("%w: invalid deck header", spec.ErrInvalid)
	}
	sections, subsections := map[string]bool{}, map[string]bool{}
	for _, section := range deck.Sections {
		if section.ID == "" || section.Title == "" || section.Purpose == "" || sections[section.ID] {
			return fmt.Errorf("%w: invalid or duplicate section", spec.ErrInvalid)
		}
		sections[section.ID] = true
		for _, subsection := range section.Subsections {
			if subsection.ID == "" || subsection.Title == "" || subsections[subsection.ID] {
				return fmt.Errorf("%w: invalid or duplicate subsection", spec.ErrInvalid)
			}
			subsections[subsection.ID] = true
		}
	}
	seen := map[string]bool{}
	for _, id := range deck.SlideOrder {
		if !stableSlideID.MatchString(id) || seen[id] {
			return fmt.Errorf("%w: invalid or duplicate slide_id %s", spec.ErrInvalid, id)
		}
		seen[id] = true
	}
	return nil
}

func validateHTML(raw []byte) ([]Issue, error) {
	issues := []Issue{}
	if len(raw) == 0 || strings.ContainsRune(string(raw), '\x00') {
		return []Issue{{Code: "HTML_PARSE_FAILED", Severity: SeverityError, Summary: "HTML is empty or contains NUL bytes"}},
			errors.New("HTML basic parsing failed")
	}
	if _, err := nethtml.Parse(strings.NewReader(string(raw))); err != nil {
		return []Issue{{Code: "HTML_PARSE_FAILED", Severity: SeverityError, Summary: err.Error()}},
			fmt.Errorf("HTML basic parsing failed: %w", err)
	}
	for _, check := range designsystem.LintSlide(raw) {
		if !check.OK {
			issues = append(issues, Issue{
				Code:     "HTML_" + strings.ToUpper(strings.ReplaceAll(check.ID, "-", "_")),
				Severity: SeverityError, Summary: check.Reason,
			})
		}
	}
	if len(issues) > 0 {
		return issues, fmt.Errorf("HTML failed %d static checks", len(issues))
	}
	return issues, nil
}

func validateReferences(pack contextengine.ContextPack, tx *RunSession) (string, error) {
	deckRaw, _, err := readArtifact(tx.ProjectDir(), tx, outlineRef(pack))
	if err != nil {
		return "", err
	}
	var deck spec.Outline
	if err := json.Unmarshal(deckRaw, &deck); err != nil {
		return "", err
	}
	slides := make(map[string]spec.SlideSpec, len(deck.SlideOrder))
	for _, id := range deck.SlideOrder {
		raw, _, err := readArtifact(tx.ProjectDir(), tx, specSlideRef(id))
		if err != nil {
			return "", fmt.Errorf("%w: missing slide %s", spec.ErrReferenceBroken, id)
		}
		var slide spec.SlideSpec
		if err := json.Unmarshal(raw, &slide); err != nil {
			return "", err
		}
		slides[id] = slide
	}
	if err := spec.ValidateOutline(deck, slides); err != nil {
		return "", err
	}
	return hashBytes(deckRaw), nil
}

func currentOutline(pack contextengine.ContextPack, tx *RunSession) (spec.Outline, error) {
	raw, _, err := readArtifact(tx.ProjectDir(), tx, outlineRef(pack))
	if err != nil {
		return spec.Outline{}, err
	}
	var deck spec.Outline
	if err := json.Unmarshal(raw, &deck); err != nil {
		return spec.Outline{}, err
	}
	return deck, nil
}

func validateSlideReference(pack contextengine.ContextPack, tx *RunSession, slide spec.SlideSpec) error {
	deck, err := currentOutline(pack, tx)
	if err != nil {
		return err
	}
	sections, subsections, ordered := map[string]bool{}, map[string]bool{}, false
	for _, section := range deck.Sections {
		sections[section.ID] = true
		for _, subsection := range section.Subsections {
			subsections[subsection.ID] = true
		}
	}
	for _, id := range deck.SlideOrder {
		ordered = ordered || id == slide.SlideID
	}
	if !ordered || !sections[slide.SectionID] || (slide.SubsectionID != "" && !subsections[slide.SubsectionID]) {
		return fmt.Errorf("%w: slide %s is not declared by the current outline", spec.ErrReferenceBroken, slide.SlideID)
	}
	return nil
}

func readSlideModel(pack contextengine.ContextPack, tx *RunSession, slideID string) (spec.SlideSpec, []byte, string, error) {
	raw, source, err := readArtifact(tx.ProjectDir(), tx, specSlideRef(slideID))
	if err != nil {
		return spec.SlideSpec{}, nil, "", err
	}
	var slide spec.SlideSpec
	if err := json.Unmarshal(raw, &slide); err != nil {
		return spec.SlideSpec{}, nil, "", err
	}
	return slide, raw, source, nil
}

func targetHash(pack contextengine.ContextPack, tx *RunSession, target Resource) (string, error) {
	ref, err := refForResource(pack, target)
	if err != nil {
		return "", err
	}
	raw, _, err := readArtifact(tx.ProjectDir(), tx, ref)
	if err != nil {
		return "", err
	}
	return hashBytes(raw), nil
}

func renderSourceHash(pack contextengine.ContextPack, tx *RunSession, slideID string) (string, error) {
	designRaw, _, err := readArtifact(tx.ProjectDir(), tx, designRef(pack))
	if err != nil {
		return "", err
	}
	specRaw, _, err := readArtifact(tx.ProjectDir(), tx, specSlideRef(slideID))
	if err != nil {
		return "", err
	}
	htmlRaw, _, err := readArtifact(tx.ProjectDir(), tx, slideHTMLRef(slideID))
	if err != nil {
		return "", err
	}
	return MaterializationSourceHash(slideID, designRaw, specRaw, htmlRaw), nil
}

// MaterializationSourceHash binds the exact Design, Slide Spec and Slide HTML
// bytes used by render and commit.
func MaterializationSourceHash(slideID string, designRaw, specRaw, htmlRaw []byte) string {
	parts := make([]byte, 0, len(designRaw)+len(specRaw)+len(htmlRaw)+128)
	for _, item := range []struct {
		key string
		raw []byte
	}{
		{key: (Resource{Type: "deck", Part: "design"}).Key(), raw: designRaw},
		{key: (Resource{Type: "slide", SlideID: slideID, Part: "spec"}).Key(), raw: specRaw},
		{key: (Resource{Type: "slide", SlideID: slideID, Part: "html"}).Key(), raw: htmlRaw},
	} {
		parts = append(parts, []byte(item.key)...)
		parts = append(parts, 0)
		parts = append(parts, item.raw...)
		parts = append(parts, 0)
	}
	return hashBytes(parts)
}

func currentMaterializationProof(
	pack contextengine.ContextPack,
	projectDir string,
	tx *RunSession,
	slideID string,
	sourceHash string,
) (MaterializationProof, error) {
	outlineRaw, _, err := readArtifact(projectDir, tx, outlineRef(pack))
	if err != nil {
		return MaterializationProof{}, err
	}
	designRaw, _, err := readArtifact(projectDir, tx, designRef(pack))
	if err != nil {
		return MaterializationProof{}, err
	}
	specRaw, _, err := readArtifact(projectDir, tx, specSlideRef(slideID))
	if err != nil {
		return MaterializationProof{}, err
	}
	var outline spec.Outline
	var design spec.Design
	var slide spec.SlideSpec
	if err := json.Unmarshal(outlineRaw, &outline); err != nil {
		return MaterializationProof{}, err
	}
	if err := json.Unmarshal(designRaw, &design); err != nil {
		return MaterializationProof{}, err
	}
	if err := json.Unmarshal(specRaw, &slide); err != nil {
		return MaterializationProof{}, err
	}
	htmlRevision := pack.Revisions.SlideHTML[slideID]
	if tx != nil && tx.HasChange(slideHTMLRef(slideID)) {
		htmlRevision++
	}
	return MaterializationProof{
		SlideID: slideID, HTMLRevision: htmlRevision,
		SourceOutlineRevision: outline.Revision,
		SourceSpecRevision:    slide.Revision,
		SourceDesignRevision:  design.Revision,
		SourceHash:            sourceHash,
	}, nil
}

func schemaEvidence(target Resource, hash string) Evidence {
	return newEvidence("schema", target, hash, map[string]any{"valid": true})
}

func staticEvidence(target Resource, hash string) Evidence {
	return newEvidence("static", target, hash, map[string]any{"valid": true})
}

func referenceEvidence(hash string) Evidence {
	return newEvidence("reference", Resource{Type: "deck", Part: "outline"}, hash, map[string]any{"valid": true})
}

func newEvidence(kind string, target Resource, sourceHash string, values ...map[string]any) Evidence {
	data := map[string]any{}
	if len(values) > 0 && values[0] != nil {
		data = values[0]
	}
	return Evidence{
		ID: fmt.Sprintf("evidence_%d_%s", time.Now().UnixNano(), kind), Kind: kind,
		Target: target, SourceHash: sourceHash, ProducedAt: time.Now().Unix(), Data: data,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func revisionFromModel(raw []byte) int {
	var header struct {
		Revision int `json:"revision"`
	}
	_ = json.Unmarshal(raw, &header)
	return header.Revision
}

func uniqueTargets(values []Resource) []Resource {
	out := []Resource{}
	seen := map[string]bool{}
	for _, value := range values {
		if value.Type == "" || seen[value.Key()] {
			continue
		}
		seen[value.Key()] = true
		out = append(out, value)
	}
	return out
}

func controlledModelPath(path string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	first := parts[0]
	if (first == "deck" || first == "design") && len(parts) > 1 {
		first = parts[1]
	}
	switch first {
	case "schema_version", "version", "revision", "project_id", "project", "slide_id", "created_at", "updated_at":
		return true
	default:
		return false
	}
}

func applyJSONEdit(doc *any, op, path string, value any) error {
	if op != "replace" && op != "add" && op != "remove" {
		return fmt.Errorf("unsupported JSON edit op %q", op)
	}
	if !strings.HasPrefix(path, "/") || path == "/" || controlledModelPath(path) {
		return fmt.Errorf("JSON edit path is not writable")
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for index := range parts {
		parts[index] = strings.ReplaceAll(strings.ReplaceAll(parts[index], "~1", "/"), "~0", "~")
	}
	next, err := editJSONValue(*doc, parts, op, value)
	if err != nil {
		return err
	}
	*doc = next
	return nil
}

func editJSONValue(current any, parts []string, op string, value any) (any, error) {
	if len(parts) == 0 {
		return nil, errors.New("JSON root replacement is not allowed")
	}
	key := parts[0]
	if len(parts) == 1 {
		switch node := current.(type) {
		case map[string]any:
			_, exists := node[key]
			if (op == "replace" || op == "remove") && !exists {
				return nil, errors.New("JSON edit path does not exist")
			}
			if op == "remove" {
				delete(node, key)
			} else {
				node[key] = value
			}
			return node, nil
		case []any:
			if key == "-" && op == "add" {
				return append(node, value), nil
			}
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index > len(node) || (op != "add" && index == len(node)) {
				return nil, errors.New("JSON array index is invalid")
			}
			switch op {
			case "add":
				node = append(node, nil)
				copy(node[index+1:], node[index:])
				node[index] = value
			case "replace":
				node[index] = value
			case "remove":
				node = append(node[:index], node[index+1:]...)
			}
			return node, nil
		default:
			return nil, errors.New("JSON edit parent is not a container")
		}
	}
	switch node := current.(type) {
	case map[string]any:
		child, ok := node[key]
		if !ok {
			return nil, errors.New("JSON edit parent does not exist")
		}
		next, err := editJSONValue(child, parts[1:], op, value)
		if err != nil {
			return nil, err
		}
		node[key] = next
		return node, nil
	case []any:
		index, err := strconv.Atoi(key)
		if err != nil || index < 0 || index >= len(node) {
			return nil, errors.New("JSON array index is invalid")
		}
		next, err := editJSONValue(node[index], parts[1:], op, value)
		if err != nil {
			return nil, err
		}
		node[index] = next
		return node, nil
	default:
		return nil, errors.New("JSON edit parent is not a container")
	}
}
