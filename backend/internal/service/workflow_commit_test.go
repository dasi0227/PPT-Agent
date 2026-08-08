package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

const commitTestHTML = `<!doctype html><html><body><section class="slide-stage">test</section></body></html>`

type commitFixture struct {
	store   *sqlitestore.Store
	project model.Project
	outline spec.Outline
	design  spec.Design
	slides  map[string]spec.SlideSpec
}

func TestWorkflowCommitMaterializationProofUpdatesSourcesWithoutHTMLRevision(t *testing.T) {
	t.Run("design only", func(t *testing.T) {
		fixture := newCommitFixture(t, 1)
		fixture.design.Revision = 2
		writeCommitJSON(t, fixture.project.WorkDir, "design.json", fixture.design)
		proof := fixture.proof(t, "slide-01", 1)
		err := (workflowCommitter{store: fixture.store, project: fixture.project, runID: "design-run"}).
			Commit(context.Background(), workflow.CommitContext{
				Changes: workflow.ChangeSet{Updated: []workflow.ArtifactChange{{
					Artifact: workflow.ArtifactRef{Kind: workflow.ArtifactDesign, ID: fixture.project.ID},
				}}},
				MaterializationProofs: []workflow.MaterializationProof{proof},
			})
		if err != nil {
			t.Fatal(err)
		}
		slide, _ := fixture.store.GetSlide(context.Background(), "slide-01")
		if slide.HTMLRevision != 1 || slide.SourceDesignRevision != 2 ||
			slide.SourceSpecRevision != 1 || slide.SourceOutlineRevision != 1 {
			t.Fatalf("design-only materialization=%+v", slide)
		}
		assertFresh(t, slide, 1, 1, 2)
	})

	t.Run("speaker notes only", func(t *testing.T) {
		fixture := newCommitFixture(t, 1)
		slideSpec := fixture.slides["slide-01"]
		slideSpec.Revision = 2
		slideSpec.SpeakerNotes = "updated notes"
		fixture.slides["slide-01"] = slideSpec
		writeCommitJSON(t, fixture.project.WorkDir, model.SlideSpecPath("slide-01"), slideSpec)
		proof := fixture.proof(t, "slide-01", 1)
		err := (workflowCommitter{store: fixture.store, project: fixture.project, runID: "spec-run"}).
			Commit(context.Background(), workflow.CommitContext{
				Changes: workflow.ChangeSet{Updated: []workflow.ArtifactChange{{
					Artifact: workflow.ArtifactRef{Kind: workflow.ArtifactSlideSpec, ID: "slide-01"},
				}}},
				MaterializationProofs: []workflow.MaterializationProof{proof},
			})
		if err != nil {
			t.Fatal(err)
		}
		slide, _ := fixture.store.GetSlide(context.Background(), "slide-01")
		if slide.HTMLRevision != 1 || slide.SpecRevision != 2 || slide.SourceSpecRevision != 2 {
			t.Fatalf("spec-only materialization=%+v", slide)
		}
		assertFresh(t, slide, 1, 2, 1)
	})
}

func TestWorkflowCommitUpdatesOnlySlidesWithAcceptedProof(t *testing.T) {
	fixture := newCommitFixture(t, 2)
	fixture.design.Revision = 2
	writeCommitJSON(t, fixture.project.WorkDir, "design.json", fixture.design)
	err := (workflowCommitter{store: fixture.store, project: fixture.project, runID: "partial-render"}).
		Commit(context.Background(), workflow.CommitContext{
			Changes: workflow.ChangeSet{Updated: []workflow.ArtifactChange{{
				Artifact: workflow.ArtifactRef{Kind: workflow.ArtifactDesign, ID: fixture.project.ID},
			}}},
			MaterializationProofs: []workflow.MaterializationProof{fixture.proof(t, "slide-01", 1)},
		})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := fixture.store.GetSlide(context.Background(), "slide-01")
	second, _ := fixture.store.GetSlide(context.Background(), "slide-02")
	if first.SourceDesignRevision != 2 || second.SourceDesignRevision != 1 {
		t.Fatalf("proof scope first=%+v second=%+v", first, second)
	}
	if state := model.DeriveMaterializationState(true, 1, 1, 2, revisions(second)); state != model.MaterializationDesignStale {
		t.Fatalf("unrendered slide state=%s", state)
	}
}

func TestWorkflowCommitHTMLChangeIncrementsHTMLRevision(t *testing.T) {
	fixture := newCommitFixture(t, 1)
	if err := os.WriteFile(filepath.Join(fixture.project.WorkDir, model.SlideHTMLPath("slide-01")),
		[]byte(commitTestHTML+"<!-- changed -->"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := (workflowCommitter{store: fixture.store, project: fixture.project, runID: "html-run"}).
		Commit(context.Background(), workflow.CommitContext{
			Changes: workflow.ChangeSet{Updated: []workflow.ArtifactChange{{
				Artifact: workflow.ArtifactRef{Kind: workflow.ArtifactSlideHTML, ID: "slide-01"},
			}}},
			MaterializationProofs: []workflow.MaterializationProof{fixture.proof(t, "slide-01", 2)},
		})
	if err != nil {
		t.Fatal(err)
	}
	slide, _ := fixture.store.GetSlide(context.Background(), "slide-01")
	if slide.HTMLRevision != 2 || slide.SourceSpecRevision != 1 || slide.SourceDesignRevision != 1 {
		t.Fatalf("HTML revision=%+v", slide)
	}
}

func TestWorkflowCommitRejectsStaleProofWithoutUpdatingSources(t *testing.T) {
	fixture := newCommitFixture(t, 1)
	stale := fixture.proof(t, "slide-01", 1)
	fixture.design.Revision = 2
	writeCommitJSON(t, fixture.project.WorkDir, "design.json", fixture.design)
	err := (workflowCommitter{store: fixture.store, project: fixture.project, runID: "stale"}).
		Commit(context.Background(), workflow.CommitContext{
			Changes: workflow.ChangeSet{Updated: []workflow.ArtifactChange{{
				Artifact: workflow.ArtifactRef{Kind: workflow.ArtifactDesign, ID: fixture.project.ID},
			}}},
			MaterializationProofs: []workflow.MaterializationProof{stale},
		})
	if err == nil {
		t.Fatal("stale proof was accepted")
	}
	slide, _ := fixture.store.GetSlide(context.Background(), "slide-01")
	if slide.SourceDesignRevision != 1 || slide.HTMLRevision != 1 {
		t.Fatalf("stale proof updated database: %+v", slide)
	}
}

func TestWorkflowCommitFailureKeepsDirectWrittenFileButLeavesDatabase(t *testing.T) {
	fixture := newCommitFixture(t, 1)
	next := fixture.design
	next.Revision = 2
	tx, err := workflow.NewRunSession(fixture.project.WorkDir, "atomic-failure")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Write(
		workflow.ArtifactRef{Kind: workflow.ArtifactDesign, ID: fixture.project.ID, Path: "design.json"},
		"write_ppt", mustJSON(next),
	); err != nil {
		t.Fatal(err)
	}
	tx.AcceptMaterializationProofs([]workflow.MaterializationProof{{
		SlideID: "slide-01", HTMLRevision: 1,
		SourceOutlineRevision: 1, SourceSpecRevision: 1, SourceDesignRevision: 2,
		SourceHash: "stale",
	}})
	committer := workflowCommitter{store: fixture.store, project: fixture.project, runID: "atomic-failure"}
	if err := tx.Commit(context.Background(), committer.Commit); err == nil {
		t.Fatal("commit with stale proof succeeded")
	}
	// Direct-write: the artifact was written to disk before finalize, so a
	// failure keeps it on disk. Only the database must stay untouched because the
	// stale proof is rejected before CommitWorkflow runs.
	written, _ := os.ReadFile(filepath.Join(fixture.project.WorkDir, "design.json"))
	project, _ := fixture.store.GetProject(context.Background(), fixture.project.ID)
	slide, _ := fixture.store.GetSlide(context.Background(), "slide-01")
	if string(written) != string(mustJSON(next)) || project.DesignRevision != 1 ||
		slide.SourceDesignRevision != 1 || slide.HTMLRevision != 1 {
		t.Fatalf("finalize failure state project=%+v slide=%+v", project, slide)
	}
}

func newCommitFixture(t *testing.T, slideCount int) commitFixture {
	t.Helper()
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(
		&config.Config{DBPath: filepath.Join(root, "commit.db")}, zap.NewNop(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project := model.Project{
		ID: "project-1", Title: "Commit", WorkDir: filepath.Join(root, "project"),
		Theme: "default", Status: "draft",
		OutlineRevision: 1, DesignRevision: 1, LayoutVersion: 2, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := os.MkdirAll(project.WorkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	outline := spec.Outline{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: project.ID,
		Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN",
		Positioning: "Thesis",
		Constraints: spec.Constraints{MustInclude: []string{}, MustAvoid: []string{}, StyleLimits: []string{}, ContentLimits: []string{}},
		Sections:    []spec.Section{{ID: "section-1", Title: "Section", Purpose: "Test section", Subsections: []spec.Subsection{}}},
		SlideOrder:  []string{}, CreatedAt: 1, UpdatedAt: 1,
	}
	design := spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: project.ID,
		Theme:     "swiss-modern",
		Direction: "test",
		Density:   "medium",
		Chrome: []spec.ChromeItem{
			{Type: "page_number", Placement: "bottom-right", Style: "tiny muted mono counter"},
		},
		CreatedAt: 1, UpdatedAt: 1,
	}
	specs := map[string]spec.SlideSpec{}
	metas := make([]model.Slide, 0, slideCount)
	for index := 1; index <= slideCount; index++ {
		id := "slide-0" + string(rune('0'+index))
		outline.SlideOrder = append(outline.SlideOrder, id)
		slideSpec := spec.SlideSpec{
			SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: project.ID, SlideID: id,
			SourceOutlineRevision: 1, SectionID: "section-1", Role: "content",
			Title: id, KeyMessage: id, Content: spec.Content{Summary: id, Points: []string{}},
			VisualIntent: spec.VisualIntent{Archetype: "content", Description: "content", AssetQueries: []string{}},
			SpeakerNotes: "", CreatedAt: 1, UpdatedAt: 1,
		}
		specs[id] = slideSpec
		writeCommitJSON(t, project.WorkDir, model.SlideSpecPath(id), slideSpec)
		if err := os.WriteFile(filepath.Join(project.WorkDir, model.SlideHTMLPath(id)), []byte(commitTestHTML), 0o644); err != nil {
			t.Fatal(err)
		}
		metas = append(metas, model.Slide{
			ID: id, ProjectID: project.ID, Position: index - 1, Layout: "content", Title: id,
			SpecPath: model.SlideSpecPath(id), HTMLPath: model.SlideHTMLPath(id),
			CurrentVersion: 1, SpecRevision: 1, HTMLRevision: 1,
			SourceOutlineRevision: 1, SourceSpecRevision: 1, SourceDesignRevision: 1,
		})
	}
	writeCommitJSON(t, project.WorkDir, "outline.json", outline)
	writeCommitJSON(t, project.WorkDir, "design.json", design)
	if err := st.ReplaceSlides(context.Background(), project.ID, metas); err != nil {
		t.Fatal(err)
	}
	return commitFixture{store: st, project: project, outline: outline, design: design, slides: specs}
}

func (f commitFixture) proof(t *testing.T, slideID string, htmlRevision int) workflow.MaterializationProof {
	t.Helper()
	designRaw, _ := os.ReadFile(filepath.Join(f.project.WorkDir, "design.json"))
	specRaw, _ := os.ReadFile(filepath.Join(f.project.WorkDir, model.SlideSpecPath(slideID)))
	htmlRaw, _ := os.ReadFile(filepath.Join(f.project.WorkDir, model.SlideHTMLPath(slideID)))
	slideSpec := f.slides[slideID]
	return workflow.MaterializationProof{
		SlideID: slideID, HTMLRevision: htmlRevision,
		SourceOutlineRevision: f.outline.Revision,
		SourceSpecRevision:    slideSpec.Revision, SourceDesignRevision: f.design.Revision,
		SourceHash: workflow.MaterializationSourceHash(slideID, designRaw, specRaw, htmlRaw),
	}
}

func writeCommitJSON(t *testing.T, root, relative string, value any) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, mustJSON(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func revisions(slide model.Slide) model.MaterializationRevisions {
	return model.MaterializationRevisions{
		SlideHTML: slide.HTMLRevision, Outline: slide.SourceOutlineRevision,
		SlideSpec: slide.SourceSpecRevision, Design: slide.SourceDesignRevision,
	}
}

func assertFresh(t *testing.T, slide model.Slide, outline, slideSpec, design int) {
	t.Helper()
	if state := model.DeriveMaterializationState(true, outline, slideSpec, design, revisions(slide)); state != model.MaterializationFresh {
		t.Fatalf("materialization state=%s revisions=%+v", state, slide)
	}
}
