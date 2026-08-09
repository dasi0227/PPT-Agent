package sqlite

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type legacyMaterializationRow struct {
	SlideID               string `gorm:"column:slide_id"`
	ProjectID             string `gorm:"column:project_id"`
	HTMLRevision          int    `gorm:"column:html_revision"`
	SourceOutlineRevision int    `gorm:"column:source_outline_revision"`
	SourceSpecRevision    int    `gorm:"column:source_spec_revision"`
	SourceDesignRevision  int    `gorm:"column:source_design_revision"`
}

func MigrateMaterializations(db *gorm.DB, log *zap.Logger) error {
	if !db.Migrator().HasTable("legacy_slide_materializations") {
		return nil
	}
	var rows []legacyMaterializationRow
	if err := db.Table("legacy_slide_materializations").Order("project_id, slide_id").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if err := migrateMaterializationRow(db, row); err != nil {
			return err
		}
	}
	if err := db.Migrator().DropTable("legacy_slide_materializations"); err != nil {
		return err
	}
	log.Info("slide materialization metadata migrated to files", zap.Int("slides", len(rows)))
	return nil
}

func migrateMaterializationRow(db *gorm.DB, row legacyMaterializationRow) error {
	if row.HTMLRevision < 1 || row.SourceOutlineRevision < 1 ||
		row.SourceSpecRevision < 1 || row.SourceDesignRevision < 1 {
		return nil
	}
	var project struct {
		WorkDir   string `gorm:"column:work_dir"`
		UpdatedAt int64  `gorm:"column:updated_at"`
	}
	if err := db.Table("projects").Select("work_dir, updated_at").Where("id = ?", row.ProjectID).Take(&project).Error; err != nil {
		return err
	}
	outlineRaw, err := os.ReadFile(filepath.Join(project.WorkDir, "outline.json"))
	if err != nil {
		return err
	}
	designRaw, err := os.ReadFile(filepath.Join(project.WorkDir, "design.json"))
	if err != nil {
		return err
	}
	specRaw, err := os.ReadFile(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(row.SlideID))))
	if err != nil {
		return err
	}
	htmlRaw, err := os.ReadFile(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(row.SlideID))))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	record := spec.MaterializationRecord{
		SchemaVersion: spec.SchemaVersion,
		Artifact: spec.MaterializationArtifact{
			Revision: row.HTMLRevision,
			Hash:     spec.ContentHash(htmlRaw),
		},
		Source: spec.MaterializationSource{
			Outline: row.SourceOutlineRevision,
			Spec:    row.SourceSpecRevision,
			Design:  row.SourceDesignRevision,
			Hash:    spec.SourceHash(outlineRaw, specRaw, designRaw),
		},
		RenderedAt: project.UpdatedAt,
	}
	if err := spec.ValidateMaterialization(record); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writeMigrationFile(
		filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideMaterializationPath(row.SlideID))),
		raw,
	)
}
