package service_test

import (
	"context"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// structureFixture seeds a project with two slides under one default section
// (direct form) and exposes everything the manual-structure tests reach for.
type structureFixture struct {
	slides    *service.SlideService
	projectID string
	sectionID string
	first     string
	second    string
}

func newStructureFixture(t *testing.T) structureFixture {
	t.Helper()
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "structure.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.NewProjectService(st, service.WorkRoot(root)).CreateProject(context.Background(), service.CreateProjectParams{
		Topic: "Structure deck", Brief: "Explain structure", Language: "zh-CN",
	})
	if err != nil {
		t.Fatal(err)
	}
	slides := service.NewSlideService(st)
	first, err := slides.AddSlide(context.Background(), project.ID, "", "content")
	if err != nil {
		t.Fatal(err)
	}
	second, err := slides.AddSlide(context.Background(), project.ID, first.ID, "content")
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.NewSpecService(st).EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	return structureFixture{
		slides:    slides,
		projectID: project.ID,
		sectionID: view.SlideSpecs[first.ID].SectionID,
		first:     first.ID,
		second:    second.ID,
	}
}

func findSection(sections []spec.Section, sectionID string) *spec.Section {
	for i := range sections {
		if sections[i].ID == sectionID {
			return &sections[i]
		}
	}
	return nil
}

// TestAddSectionAppendsDirectSection verifies a new section starts direct/empty.
func TestAddSectionAppendsDirectSection(t *testing.T) {
	fx := newStructureFixture(t)
	snapshot, err := fx.slides.AddSection(context.Background(), fx.projectID, "新增章节标题")
	if err != nil {
		t.Fatal(err)
	}
	sections := snapshot.Spec.Outline.Sections
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	added := sections[len(sections)-1]
	if added.Title != "新增章节标题" || len(added.Subsections) != 0 {
		t.Fatalf("unexpected added section %+v", added)
	}
}

// TestAddSubsectionMigratesDirectPages checks that turning a direct section into
// a grouped one moves its existing direct pages into the new subsection.
func TestAddSubsectionMigratesDirectPages(t *testing.T) {
	fx := newStructureFixture(t)
	snapshot, err := fx.slides.AddSubsection(context.Background(), fx.projectID, fx.sectionID, "细分子节")
	if err != nil {
		t.Fatal(err)
	}
	section := findSection(snapshot.Spec.Outline.Sections, fx.sectionID)
	if section == nil || len(section.Subsections) != 1 {
		t.Fatalf("expected one subsection, got %+v", section)
	}
	subID := section.Subsections[0].ID
	for _, id := range []string{fx.first, fx.second} {
		if got := snapshot.Spec.SlideSpecs[id].SubsectionID; got != subID {
			t.Fatalf("slide %s expected subsection %s, got %q", id, subID, got)
		}
	}
}

// TestRemoveSectionRejectedWhileOwningPages guards that a section holding pages
// cannot be deleted; the user must move pages out first.
func TestRemoveSectionRejectedWhileOwningPages(t *testing.T) {
	fx := newStructureFixture(t)
	if _, err := fx.slides.RemoveSection(context.Background(), fx.projectID, fx.sectionID); err == nil {
		t.Fatal("removing a section that still owns pages should fail")
	} else if !service.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

// TestRemoveSoleSubsectionFallsBackToDirect verifies deleting the only
// subsection of a grouped section drops its pages back to direct form.
func TestRemoveSoleSubsectionFallsBackToDirect(t *testing.T) {
	fx := newStructureFixture(t)
	grouped, err := fx.slides.AddSubsection(context.Background(), fx.projectID, fx.sectionID, "唯一子节")
	if err != nil {
		t.Fatal(err)
	}
	subID := findSection(grouped.Spec.Outline.Sections, fx.sectionID).Subsections[0].ID
	snapshot, err := fx.slides.RemoveSubsection(context.Background(), fx.projectID, fx.sectionID, subID)
	if err != nil {
		t.Fatal(err)
	}
	section := findSection(snapshot.Spec.Outline.Sections, fx.sectionID)
	if section == nil || len(section.Subsections) != 0 {
		t.Fatalf("expected section back to direct form, got %+v", section)
	}
	for _, id := range []string{fx.first, fx.second} {
		if got := snapshot.Spec.SlideSpecs[id].SubsectionID; got != "" {
			t.Fatalf("slide %s should be direct, got subsection %q", id, got)
		}
	}
}
