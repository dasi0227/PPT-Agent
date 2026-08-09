package sqlite

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
	"github.com/dasi0227/PPT-Agent/backend/seed"
)

const currentProjectLayoutVersion = 5

type layoutProjectRow struct {
	ID            string `gorm:"column:id"`
	WorkDir       string `gorm:"column:work_dir"`
	CreatedAt     int64  `gorm:"column:created_at"`
	UpdatedAt     int64  `gorm:"column:updated_at"`
	LayoutVersion int    `gorm:"column:layout_version"`
}

type layoutSlideRow struct {
	ID string `gorm:"column:id"`
}

type layoutVersionRow struct {
	ID           string `gorm:"column:id"`
	TargetType   string `gorm:"column:target_type"`
	TargetID     string `gorm:"column:target_id"`
	SnapshotPath string `gorm:"column:snapshot_path"`
}

type layoutFile struct {
	sourceRel string
	destRel   string
	content   []byte
}

type appliedLayoutFile struct {
	sourceAbs string
	destAbs   string
	backupAbs string
}

// MigrateProjectLayouts performs the one-time filesystem half of schema 0002.
// A project is marked current only after all files and metadata are updated.
// Any error restores every file moved for that project and stops startup.
func MigrateProjectLayouts(db *gorm.DB, log *zap.Logger) error {
	var projects []layoutProjectRow
	if err := db.Table("projects").Where("layout_version < ?", currentProjectLayoutVersion).Find(&projects).Error; err != nil {
		return err
	}
	for _, project := range projects {
		if err := migrateProjectLayout(db, project); err != nil {
			return fmt.Errorf("migrate project %s layout: %w", project.ID, err)
		}
		log.Info("project resource layout migrated", zap.String("project_id", project.ID))
	}
	return nil
}

func migrateProjectLayout(db *gorm.DB, project layoutProjectRow) error {
	var slides []layoutSlideRow
	if err := db.Table("slides").Where("project_id = ?", project.ID).Find(&slides).Error; err != nil {
		return err
	}
	var versions []layoutVersionRow
	prefix := "project/" + project.ID + "/%"
	if err := db.Table("versions").Where("target_id LIKE ?", prefix).Find(&versions).Error; err != nil {
		return err
	}
	files, versionUpdates, err := prepareLayoutFiles(project, slides, versions)
	if err != nil {
		return err
	}
	applied, cleanup, err := applyLayoutFiles(project.WorkDir, files)
	if err != nil {
		return err
	}
	rollback := func() {
		for i := len(applied) - 1; i >= 0; i-- {
			item := applied[i]
			_ = os.Remove(item.destAbs)
			if item.backupAbs != "" {
				_ = os.MkdirAll(filepath.Dir(item.sourceAbs), 0o755)
				_ = os.Rename(item.backupAbs, item.sourceAbs)
			}
		}
		cleanup()
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		// outline_path/design_path/spec_path columns were removed once files
		// became the single source of truth; only the layout_version cursor and
		// the version-table remapping remain to persist here.
		if err := tx.Table("projects").Where("id = ?", project.ID).Updates(map[string]any{
			"layout_version": currentProjectLayoutVersion,
		}).Error; err != nil {
			return err
		}
		for _, update := range versionUpdates {
			if err := tx.Table("versions").Where("id = ?", update.ID).Updates(map[string]any{
				"target_id": update.TargetID, "snapshot_path": update.SnapshotPath,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		rollback()
		return err
	}
	cleanup()
	removeEmptyLegacyDirs(project.WorkDir, slides)
	return nil
}

func prepareLayoutFiles(
	project layoutProjectRow,
	slides []layoutSlideRow,
	versions []layoutVersionRow,
) ([]layoutFile, []layoutVersionRow, error) {
	files := []layoutFile{}
	var originalOutline []byte
	var originalDesign []byte
	originalSpecs := map[string][]byte{}
	var normalizedOutline []byte
	var normalizedDesign []byte
	normalizedSpecs := map[string][]byte{}
	addJSON := func(oldRel, newRel, kind string, revision int, slideID string) error {
		sourceRel, raw, err := readLegacyOrNew(project.WorkDir, oldRel, newRel)
		if err != nil {
			return err
		}
		normalized, err := migrateResourceJSON(raw, kind, project, revision, slideID)
		if err != nil {
			return fmt.Errorf("%s: %w", sourceRel, err)
		}
		files = append(files, layoutFile{sourceRel: sourceRel, destRel: newRel, content: normalized})
		switch kind {
		case "outline":
			originalOutline = raw
			normalizedOutline = normalized
		case "design":
			originalDesign = raw
			normalizedDesign = normalized
		case "slide_spec":
			originalSpecs[slideID] = raw
			normalizedSpecs[slideID] = normalized
		}
		return nil
	}
	if err := addJSON("deck.json", "outline.json", "outline", 0, ""); err != nil {
		return nil, nil, err
	}
	if err := addJSON("design/design-spec.json", "design.json", "design", 1, ""); err != nil {
		return nil, nil, err
	}
	for _, slide := range slides {
		if err := addJSON(
			"slides/"+slide.ID+"/slide.json",
			model.SlideSpecPath(slide.ID),
			"slide_spec",
			0,
			slide.ID,
		); err != nil {
			return nil, nil, err
		}
	}
	var design spec.Design
	if err := json.Unmarshal(normalizedDesign, &design); err != nil {
		return nil, nil, err
	}
	baseCSS, err := fs.ReadFile(seed.FS(), "common/base.css")
	if err != nil {
		return nil, nil, err
	}
	files = append(files,
		derivedLayoutFile(project.WorkDir, "common/base.css", baseCSS),
		derivedLayoutFile(project.WorkDir, "common/tokens.css", spec.DesignTokensCSS(design)),
	)
	var outline spec.Outline
	if err := json.Unmarshal(normalizedOutline, &outline); err != nil {
		return nil, nil, err
	}
	for _, slide := range slides {
		var slideSpec spec.SlideSpec
		if err := json.Unmarshal(normalizedSpecs[slide.ID], &slideSpec); err != nil {
			return nil, nil, err
		}
		materializationPath := filepath.Join(
			project.WorkDir,
			filepath.FromSlash(model.SlideMaterializationPath(slide.ID)),
		)
		record, err := spec.ReadMaterialization(materializationPath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			continue
		}
		if record.Source.Outline != outline.Revision ||
			record.Source.Spec != slideSpec.Revision ||
			record.Source.Design != design.Revision {
			continue
		}
		if record.Source.Hash != spec.SourceHash(originalOutline, originalSpecs[slide.ID], originalDesign) {
			continue
		}
		record.Source.Hash = spec.SourceHash(normalizedOutline, normalizedSpecs[slide.ID], normalizedDesign)
		raw, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return nil, nil, err
		}
		files = append(files, derivedLayoutFile(
			project.WorkDir,
			model.SlideMaterializationPath(slide.ID),
			raw,
		))
	}

	updates := make([]layoutVersionRow, 0, len(versions))
	for _, version := range versions {
		next := version
		switch version.TargetType {
		case "outline":
			next.TargetID = model.OutlineVersionTarget(project.ID)
			next.SnapshotPath = model.OutlineVersionSnapshot(versionNumber(version.SnapshotPath))
			sourceRel, raw, err := readLegacyOrNew(project.WorkDir, version.SnapshotPath, next.SnapshotPath)
			if err != nil {
				return nil, nil, err
			}
			normalized, err := migrateResourceJSON(raw, "outline", project, versionNumber(version.SnapshotPath), "")
			if err != nil {
				return nil, nil, err
			}
			files = append(files, layoutFile{sourceRel: sourceRel, destRel: next.SnapshotPath, content: normalized})
		case "slide_spec":
			slideID := slideIDFromTarget(version.TargetID, "blueprint-slide-", "slide-spec-")
			next.TargetID = model.SlideSpecVersionTarget(project.ID, slideID)
			next.SnapshotPath = model.SlideSpecVersionSnapshot(slideID, versionNumber(version.SnapshotPath))
			sourceRel, raw, err := readLegacyOrNew(project.WorkDir, version.SnapshotPath, next.SnapshotPath)
			if err != nil {
				return nil, nil, err
			}
			normalized, err := migrateResourceJSON(raw, "slide_spec", project, versionNumber(version.SnapshotPath), slideID)
			if err != nil {
				return nil, nil, err
			}
			files = append(files, layoutFile{sourceRel: sourceRel, destRel: next.SnapshotPath, content: normalized})
		case "slide_html":
			slideID := slideIDFromTarget(version.TargetID, "slide-", "slide-html-")
			next.TargetID = model.SlideHTMLVersionTarget(project.ID, slideID)
			next.SnapshotPath = model.SlideHTMLVersionSnapshot(slideID, versionNumber(version.SnapshotPath))
			sourceRel, raw, err := readLegacyOrNew(project.WorkDir, version.SnapshotPath, next.SnapshotPath)
			if err != nil {
				return nil, nil, err
			}
			files = append(files, layoutFile{sourceRel: sourceRel, destRel: next.SnapshotPath, content: raw})
		case "design":
			sourceRel, raw, err := readLegacyOrNew(project.WorkDir, version.SnapshotPath, version.SnapshotPath)
			if err != nil {
				return nil, nil, err
			}
			normalized, err := migrateResourceJSON(raw, "design", project, versionNumber(version.SnapshotPath), "")
			if err != nil {
				return nil, nil, err
			}
			files = append(files, layoutFile{sourceRel: sourceRel, destRel: version.SnapshotPath, content: normalized})
		}
		updates = append(updates, next)
	}
	return dedupeLayoutFiles(files), updates, nil
}

func derivedLayoutFile(workDir, relative string, content []byte) layoutFile {
	source := ""
	if _, err := os.Stat(filepath.Join(workDir, filepath.FromSlash(relative))); err == nil {
		source = relative
	}
	return layoutFile{sourceRel: source, destRel: relative, content: content}
}

func readLegacyOrNew(workDir, oldRel, newRel string) (string, []byte, error) {
	oldRaw, oldErr := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(oldRel)))
	newRaw, newErr := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(newRel)))
	if oldRel != newRel && oldErr == nil && newErr == nil {
		return "", nil, fmt.Errorf("both legacy and new resource exist: %s and %s", oldRel, newRel)
	}
	if oldErr == nil {
		return oldRel, oldRaw, nil
	}
	if newErr == nil {
		return newRel, newRaw, nil
	}
	if !errors.Is(oldErr, fs.ErrNotExist) {
		return "", nil, oldErr
	}
	if !errors.Is(newErr, fs.ErrNotExist) {
		return "", nil, newErr
	}
	return "", nil, fmt.Errorf("resource not found: %s", oldRel)
}

func migrateResourceJSON(raw []byte, kind string, project layoutProjectRow, revision int, slideID string) ([]byte, error) {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	value["revision"] = maxMigrationInt(revision, intValueFromJSON(value["revision"], 1))
	value["created_at"] = int64ValueFromJSON(value["created_at"], project.CreatedAt)
	value["updated_at"] = int64ValueFromJSON(value["updated_at"], project.UpdatedAt)
	switch kind {
	case "outline":
		value["version"] = spec.SchemaVersion
		value["project_id"] = project.ID
		delete(value, "schema_version")
		delete(value, "project")
		if positioning, ok := value["positioning"].(string); !ok || strings.TrimSpace(positioning) == "" {
			if thesis, ok := value["core_thesis"].(string); ok && strings.TrimSpace(thesis) != "" {
				value["positioning"] = thesis
			}
		}
		delete(value, "core_thesis")
		delete(value, "narrative_arc")
		constraints, _ := value["constraints"].(map[string]any)
		if _, ok := value["requirements"].([]any); !ok {
			value["requirements"] = migrationStringList(
				constraints["must_include"],
				constraints["style_limits"],
				constraints["content_limits"],
			)
		}
		if _, ok := value["prohibitions"].([]any); !ok {
			value["prohibitions"] = migrationStringList(constraints["must_avoid"])
		}
		delete(value, "constraints")
		if sections, ok := value["sections"].([]any); ok {
			for _, rawSection := range sections {
				section, ok := rawSection.(map[string]any)
				if !ok {
					continue
				}
				delete(section, "number")
				if purpose, ok := section["purpose"].(string); !ok || strings.TrimSpace(purpose) == "" {
					title, _ := section["title"].(string)
					section["purpose"] = firstMigrationText(title, "组织本章节内容")
				}
				if subsections, ok := section["subsections"].([]any); ok {
					for _, rawSubsection := range subsections {
						if subsection, ok := rawSubsection.(map[string]any); ok {
							delete(subsection, "number")
						}
					}
				}
			}
		}
	case "design":
		value["version"] = spec.SchemaVersion
		value["project_id"] = project.ID
		delete(value, "schema_version")
		delete(value, "project")
		value["theme"] = firstMigrationText(stringValueFromJSON(value["theme"]), "swiss-modern")
		value["direction"] = firstMigrationText(stringValueFromJSON(value["direction"]), firstMigrationText(stringValueFromJSON(value["signature"]), "清晰、克制、结构化的通用商务演示"))
		if density := stringValueFromJSON(value["density"]); density != "" {
			value["density"] = density
		} else if layoutSystem, ok := value["layout_system"].(map[string]any); ok {
			value["density"] = firstMigrationText(stringValueFromJSON(layoutSystem["density"]), "medium")
		} else {
			value["density"] = "medium"
		}
		if _, ok := value["chrome"].([]any); !ok {
			value["chrome"] = []any{
				map[string]any{"type": "page_number", "placement": "bottom-right", "style": "tiny muted mono counter"},
				map[string]any{"type": "section_marker", "placement": "top-left", "style": "compact section label"},
			}
		}
		for _, removed := range []string{"canvas", "palette", "typography", "spacing", "radius", "shadows", "layout_system", "signature", "motion"} {
			delete(value, removed)
		}
	case "slide_spec":
		value["version"] = spec.SchemaVersion
		value["project_id"] = project.ID
		value["slide_id"] = slideID
		delete(value, "schema_version")
		delete(value, "project")
		delete(value, "source_outline_revision")
		delete(value, "speaker_notes")
		if layout := stringValueFromJSON(value["layout"]); strings.TrimSpace(layout) == "" {
			if visual, ok := value["visual_intent"].(map[string]any); ok {
				if archetype := stringValueFromJSON(visual["archetype"]); strings.TrimSpace(archetype) != "" {
					value["layout"] = archetype
				}
			}
		}
		if _, ok := value["elements"].([]any); !ok {
			elements := []any{}
			if content, ok := value["content"].(map[string]any); ok {
				if summary := stringValueFromJSON(content["summary"]); strings.TrimSpace(summary) != "" {
					elements = append(elements, map[string]any{"type": "text", "intent": summary})
				}
				if points, ok := content["points"].([]any); ok {
					texts := []string{}
					for _, point := range points {
						if text, ok := point.(string); ok && strings.TrimSpace(text) != "" {
							texts = append(texts, text)
						}
					}
					if len(texts) > 0 {
						elements = append(elements, map[string]any{
							"type": "list", "intent": strings.Join(texts, "；"),
						})
					}
				}
			}
			if visual, ok := value["visual_intent"].(map[string]any); ok {
				if queries, ok := visual["asset_queries"].([]any); ok {
					for _, query := range queries {
						if text, ok := query.(string); ok && strings.TrimSpace(text) != "" {
							elements = append(elements, map[string]any{"type": "asset", "intent": text})
						}
					}
				}
			}
			if len(elements) == 0 {
				elements = append(elements, map[string]any{
					"type": "text", "intent": firstMigrationText(stringValueFromJSON(value["key_message"]), stringValueFromJSON(value["title"])),
				})
			}
			value["elements"] = elements
		}
		delete(value, "content")
		delete(value, "visual_intent")
	default:
		return nil, fmt.Errorf("unknown resource kind %q", kind)
	}
	normalized, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	schemaName := map[string]string{
		"outline": pptschema.OutlineName, "design": pptschema.DesignName, "slide_spec": pptschema.SlideSpecName,
	}[kind]
	if err := pptschema.ValidateJSON(schemaName, normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func applyLayoutFiles(workDir string, files []layoutFile) ([]appliedLayoutFile, func(), error) {
	tempRoot, err := os.MkdirTemp(workDir, ".layout-v2-")
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tempRoot) }
	for index, file := range files {
		staged := filepath.Join(tempRoot, "staged", fmt.Sprintf("%04d", index))
		if err := writeMigrationFile(staged, file.content); err != nil {
			cleanup()
			return nil, func() {}, err
		}
	}
	applied := make([]appliedLayoutFile, 0, len(files))
	rollback := func() {
		for i := len(applied) - 1; i >= 0; i-- {
			item := applied[i]
			_ = os.Remove(item.destAbs)
			if item.backupAbs != "" {
				_ = os.MkdirAll(filepath.Dir(item.sourceAbs), 0o755)
				_ = os.Rename(item.backupAbs, item.sourceAbs)
			}
		}
		cleanup()
	}
	for index, file := range files {
		sourceAbs := ""
		if file.sourceRel != "" {
			sourceAbs = filepath.Join(workDir, filepath.FromSlash(file.sourceRel))
		}
		destAbs := filepath.Join(workDir, filepath.FromSlash(file.destRel))
		backupAbs := filepath.Join(tempRoot, "backup", fmt.Sprintf("%04d", index))
		if sourceAbs != "" {
			if err := os.MkdirAll(filepath.Dir(backupAbs), 0o755); err != nil {
				rollback()
				return nil, func() {}, err
			}
			if err := os.Rename(sourceAbs, backupAbs); err != nil {
				rollback()
				return nil, func() {}, err
			}
		} else {
			backupAbs = ""
		}
		if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
			if backupAbs != "" {
				_ = os.Rename(backupAbs, sourceAbs)
			}
			rollback()
			return nil, func() {}, err
		}
		staged := filepath.Join(tempRoot, "staged", fmt.Sprintf("%04d", index))
		if err := os.Rename(staged, destAbs); err != nil {
			if backupAbs != "" {
				_ = os.Rename(backupAbs, sourceAbs)
			}
			rollback()
			return nil, func() {}, err
		}
		applied = append(applied, appliedLayoutFile{sourceAbs: sourceAbs, destAbs: destAbs, backupAbs: backupAbs})
	}
	return applied, cleanup, nil
}

func writeMigrationFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(content); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func dedupeLayoutFiles(files []layoutFile) []layoutFile {
	seen := map[string]bool{}
	out := make([]layoutFile, 0, len(files))
	for _, file := range files {
		key := file.sourceRel + "\x00" + file.destRel
		if !seen[key] {
			seen[key] = true
			out = append(out, file)
		}
	}
	return out
}

func removeEmptyLegacyDirs(workDir string, slides []layoutSlideRow) {
	for _, slide := range slides {
		_ = os.Remove(filepath.Join(workDir, "slides", slide.ID))
	}
	_ = os.Remove(filepath.Join(workDir, "design"))
	for _, rel := range []string{"versions/blueprint-deck"} {
		_ = os.Remove(filepath.Join(workDir, filepath.FromSlash(rel)))
	}
}

func versionNumber(path string) int {
	base := filepath.Base(path)
	var value int
	_, _ = fmt.Sscanf(base, "v%d", &value)
	if value < 1 {
		value = 1
	}
	return value
}

func slideIDFromTarget(target string, legacyPrefix, currentPrefix string) string {
	base := target[strings.LastIndex(target, "/")+1:]
	for _, prefix := range []string{currentPrefix, legacyPrefix} {
		if strings.HasPrefix(base, prefix) {
			return strings.TrimPrefix(base, prefix)
		}
	}
	return base
}

func intValueFromJSON(value any, fallback int) int {
	if number, ok := value.(float64); ok && number >= 1 {
		return int(number)
	}
	return fallback
}

func int64ValueFromJSON(value any, fallback int64) int64 {
	if number, ok := value.(float64); ok && number >= 0 {
		return int64(number)
	}
	return fallback
}

func stringValueFromJSON(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func migrationStringList(values ...any) []any {
	result := []any{}
	seen := map[string]bool{}
	for _, value := range values {
		items, _ := value.([]any)
		for _, item := range items {
			text := stringValueFromJSON(item)
			if text != "" && !seen[text] {
				result = append(result, text)
				seen[text] = true
			}
		}
	}
	return result
}

func firstMigrationText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxMigrationInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
