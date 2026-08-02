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

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	nethtml "golang.org/x/net/html"
)

var stableSlideID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func deckRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactDeck, ID: pack.Project.ID, Path: "deck.json", Project: pack.Project.ID}
}

func designRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactDesign, ID: pack.Project.ID, Path: "design/design-spec.json", Project: pack.Project.ID}
}

func blueprintSlideRef(id string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactSlide, ID: id, Path: model.SlideJSONPath(id)}
}

func presentationSlideRef(id string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactPresentation, ID: id, Path: model.SlideHTMLPath(id)}
}

func refsForTarget(pack contextengine.ContextPack, target TargetRef) ([]ArtifactRef, error) {
	switch target.Type {
	case "global":
		if pack.WorkSpec.Target.Artifact == model.ArtifactBlueprint {
			return []ArtifactRef{deckRef(pack)}, nil
		}
		return []ArtifactRef{deckRef(pack), designRef(pack)}, nil
	case "slide":
		if !stableSlideID.MatchString(target.SlideID) || target.SlideID == "current" {
			return nil, fmt.Errorf("slide_id must be a stable opaque slide identifier")
		}
		if pack.WorkSpec.Target.Artifact == model.ArtifactBlueprint {
			return []ArtifactRef{blueprintSlideRef(target.SlideID)}, nil
		}
		return []ArtifactRef{blueprintSlideRef(target.SlideID), presentationSlideRef(target.SlideID)}, nil
	default:
		return nil, fmt.Errorf("target type must be global or slide")
	}
}

func parseTarget(args map[string]any) (TargetRef, error) {
	value, ok := args["target"].(map[string]any)
	if !ok {
		return TargetRef{}, fmt.Errorf("target must be an object")
	}
	target := TargetRef{Type: stringValue(value["type"]), SlideID: stringValue(value["slide_id"])}
	if target.Type == "global" && target.SlideID != "" {
		return TargetRef{}, fmt.Errorf("global target forbids slide_id")
	}
	if target.Type == "slide" {
		if target.SlideID == "" {
			return TargetRef{}, fmt.Errorf("slide target requires slide_id")
		}
		if !stableSlideID.MatchString(target.SlideID) || target.SlideID == "current" {
			return TargetRef{}, fmt.Errorf("slide_id must be stable and must not contain a path")
		}
	}
	if target.Type != "global" && target.Type != "slide" {
		return TargetRef{}, fmt.Errorf("target type must be global or slide")
	}
	return target, nil
}

func readArtifact(projectDir string, tx *Transaction, ref ArtifactRef) ([]byte, string, error) {
	if tx != nil {
		raw, err := tx.Read(ref)
		if err == nil {
			source := "committed"
			if tx.IsStaged(ref) {
				source = "staged"
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

func stagingFailure(err error) ToolResult {
	if errors.Is(err, ErrStagedHashMismatch) {
		return failedToolResult(CodeRevisionConflict, err.Error(), true)
	}
	return failedToolResult("STAGING_FAILED", err.Error(), true)
}

func targetSchema() map[string]any {
	return map[string]any{
		"oneOf": []any{
			objectSchema([]string{"type"}, map[string]any{
				"type": map[string]any{"const": "global"},
			}),
			objectSchema([]string{"type", "slide_id"}, map[string]any{
				"type": map[string]any{"const": "slide"},
				"slide_id": map[string]any{
					"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`,
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

func contentMap(args map[string]any) (map[string]any, error) {
	content, ok := args["content"].(map[string]any)
	if !ok {
		return nil, errors.New("content must be an object")
	}
	return content, nil
}

func normalizeModel(pack contextengine.ContextPack, tx *Transaction, ref ArtifactRef, value any) ([]byte, int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, 0, err
	}
	now := time.Now().Unix()
	switch ref.Kind {
	case ArtifactDeck:
		var next blueprint.Deck
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, 0, err
		}
		var current blueprint.Deck
		currentRaw, _, readErr := readArtifact(tx.ProjectDir(), tx, ref)
		if readErr == nil {
			_ = json.Unmarshal(currentRaw, &current)
		}
		next.SchemaVersion = blueprint.SchemaVersion
		next.ProjectID = pack.Project.ID
		next.Revision = maxInt(current.Revision+1, 1)
		if current.CreatedAt != 0 {
			next.CreatedAt = current.CreatedAt
		} else {
			next.CreatedAt = now
		}
		next.UpdatedAt = now
		if err := validateDeckStructure(next); err != nil {
			return nil, 0, err
		}
		raw, _ = json.MarshalIndent(next, "", "  ")
		return raw, next.Revision, nil
	case ArtifactSlide:
		var next blueprint.Slide
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, 0, err
		}
		var current blueprint.Slide
		currentRaw, _, readErr := readArtifact(tx.ProjectDir(), tx, ref)
		if readErr == nil {
			_ = json.Unmarshal(currentRaw, &current)
		}
		next.SchemaVersion = blueprint.SchemaVersion
		next.SlideID = ref.ID
		next.Revision = maxInt(current.Revision+1, 1)
		if current.CreatedAt != 0 {
			next.CreatedAt = current.CreatedAt
		} else {
			next.CreatedAt = now
		}
		next.UpdatedAt = now
		if err := blueprint.ValidateSlide(next); err != nil {
			return nil, 0, err
		}
		raw, _ = json.MarshalIndent(next, "", "  ")
		return raw, next.Revision, nil
	case ArtifactDesign:
		var next blueprint.DesignSpec
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, 0, err
		}
		var current blueprint.DesignSpec
		currentRaw, _, readErr := readArtifact(tx.ProjectDir(), tx, ref)
		if readErr == nil {
			_ = json.Unmarshal(currentRaw, &current)
		}
		next.SchemaVersion = blueprint.SchemaVersion
		next.Revision = maxInt(current.Revision+1, 1)
		if err := blueprint.ValidateDesignSpec(next); err != nil {
			return nil, 0, err
		}
		if next.Typography.Utility.Family == "" || next.Spacing.Unit <= 0 ||
			next.Shadows.Card == "" || next.LayoutSystem.Density == "" || next.Motion.Policy == "" {
			return nil, 0, fmt.Errorf("%w: incomplete design spec", blueprint.ErrInvalid)
		}
		raw, _ = json.MarshalIndent(next, "", "  ")
		return raw, next.Revision, nil
	default:
		return nil, 0, errors.New("target does not accept a JSON model")
	}
}

func validateDeckStructure(deck blueprint.Deck) error {
	if deck.SchemaVersion != blueprint.SchemaVersion || deck.Revision < 1 ||
		deck.ProjectID == "" || strings.TrimSpace(deck.Title) == "" ||
		strings.TrimSpace(deck.Goal) == "" || strings.TrimSpace(deck.Audience) == "" ||
		strings.TrimSpace(deck.Language) == "" || strings.TrimSpace(deck.CoreThesis) == "" ||
		strings.TrimSpace(deck.NarrativeArc) == "" || deck.Sections == nil || deck.SlideOrder == nil {
		return fmt.Errorf("%w: invalid deck header", blueprint.ErrInvalid)
	}
	sections, subsections := map[string]bool{}, map[string]bool{}
	for _, section := range deck.Sections {
		if section.ID == "" || section.Number == "" || section.Title == "" || sections[section.ID] {
			return fmt.Errorf("%w: invalid or duplicate section", blueprint.ErrInvalid)
		}
		sections[section.ID] = true
		for _, subsection := range section.Subsections {
			if subsection.ID == "" || subsection.Number == "" || subsection.Title == "" || subsections[subsection.ID] {
				return fmt.Errorf("%w: invalid or duplicate subsection", blueprint.ErrInvalid)
			}
			subsections[subsection.ID] = true
		}
	}
	seen := map[string]bool{}
	for _, id := range deck.SlideOrder {
		if !stableSlideID.MatchString(id) || seen[id] {
			return fmt.Errorf("%w: invalid or duplicate slide_id %s", blueprint.ErrInvalid, id)
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

func validateReferences(pack contextengine.ContextPack, tx *Transaction) (string, error) {
	deckRaw, _, err := readArtifact(tx.ProjectDir(), tx, deckRef(pack))
	if err != nil {
		return "", err
	}
	var deck blueprint.Deck
	if err := json.Unmarshal(deckRaw, &deck); err != nil {
		return "", err
	}
	slides := make(map[string]blueprint.Slide, len(deck.SlideOrder))
	for _, id := range deck.SlideOrder {
		raw, _, err := readArtifact(tx.ProjectDir(), tx, blueprintSlideRef(id))
		if err != nil {
			return "", fmt.Errorf("%w: missing slide %s", blueprint.ErrReferenceBroken, id)
		}
		var slide blueprint.Slide
		if err := json.Unmarshal(raw, &slide); err != nil {
			return "", err
		}
		slides[id] = slide
	}
	if err := blueprint.ValidateDeck(deck, slides); err != nil {
		return "", err
	}
	return hashBytes(deckRaw), nil
}

func currentDeck(pack contextengine.ContextPack, tx *Transaction) (blueprint.Deck, error) {
	raw, _, err := readArtifact(tx.ProjectDir(), tx, deckRef(pack))
	if err != nil {
		return blueprint.Deck{}, err
	}
	var deck blueprint.Deck
	if err := json.Unmarshal(raw, &deck); err != nil {
		return blueprint.Deck{}, err
	}
	return deck, nil
}

func validateSlideReference(pack contextengine.ContextPack, tx *Transaction, slide blueprint.Slide) error {
	deck, err := currentDeck(pack, tx)
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
		return fmt.Errorf("%w: slide %s is not declared by the staged global model", blueprint.ErrReferenceBroken, slide.SlideID)
	}
	return nil
}

func readSlideModel(pack contextengine.ContextPack, tx *Transaction, slideID string) (blueprint.Slide, []byte, string, error) {
	raw, source, err := readArtifact(tx.ProjectDir(), tx, blueprintSlideRef(slideID))
	if err != nil {
		return blueprint.Slide{}, nil, "", err
	}
	var slide blueprint.Slide
	if err := json.Unmarshal(raw, &slide); err != nil {
		return blueprint.Slide{}, nil, "", err
	}
	return slide, raw, source, nil
}

func targetHash(pack contextengine.ContextPack, tx *Transaction, target TargetRef) (string, error) {
	refs, err := refsForTarget(pack, target)
	if err != nil {
		return "", err
	}
	parts := make([]byte, 0)
	for _, ref := range refs {
		raw, _, readErr := readArtifact(tx.ProjectDir(), tx, ref)
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

func renderSourceHash(pack contextengine.ContextPack, tx *Transaction, slideID string) (string, error) {
	parts := make([]byte, 0)
	for _, ref := range []ArtifactRef{designRef(pack), blueprintSlideRef(slideID), presentationSlideRef(slideID)} {
		raw, _, err := readArtifact(tx.ProjectDir(), tx, ref)
		if err != nil {
			return "", err
		}
		parts = append(parts, []byte(ref.Key())...)
		parts = append(parts, 0)
		parts = append(parts, raw...)
		parts = append(parts, 0)
	}
	return hashBytes(parts), nil
}

func schemaEvidence(target TargetRef, hash string) Evidence {
	return newEvidence("schema", target, hash, map[string]any{"valid": true})
}

func staticEvidence(target TargetRef, hash string) Evidence {
	return newEvidence("static", target, hash, map[string]any{"valid": true})
}

func referenceEvidence(hash string) Evidence {
	return newEvidence("reference", TargetRef{Type: "global"}, hash, map[string]any{"valid": true})
}

func newEvidence(kind string, target TargetRef, sourceHash string, values ...map[string]any) Evidence {
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

func uniqueTargets(values []TargetRef) []TargetRef {
	out := []TargetRef{}
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
	case "schema_version", "revision", "project_id", "slide_id", "created_at", "updated_at":
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
