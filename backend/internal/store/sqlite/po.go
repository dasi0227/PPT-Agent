package sqlite

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// 持久化对象（PO）：GORM tag 仅出现在本包（ARCH-BACKEND-006）。PO↔model 在 store 边界互转。

type projectPO struct {
	ID             string `gorm:"column:id;primaryKey"`
	Title          string `gorm:"column:title"`
	WorkDir        string `gorm:"column:work_dir"`
	Theme          string `gorm:"column:theme"`
	Status         string `gorm:"column:status"`
	DesignPath     string `gorm:"column:design_path"`
	DeckPath       string `gorm:"column:deck_path"`
	DeckRevision   int    `gorm:"column:deck_revision"`
	DesignRevision int    `gorm:"column:design_revision"`
	CreatedAt      int64  `gorm:"column:created_at"`
	UpdatedAt      int64  `gorm:"column:updated_at"`
}

func (projectPO) TableName() string { return "projects" }

func (p projectPO) toModel() model.Project {
	return model.Project{
		ID: p.ID, Title: p.Title, WorkDir: p.WorkDir, Theme: p.Theme,
		Status: p.Status, DesignPath: p.DesignPath, DeckPath: p.DeckPath,
		DeckRevision: p.DeckRevision, DesignRevision: p.DesignRevision,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func projectToPO(m model.Project) projectPO {
	return projectPO{
		ID: m.ID, Title: m.Title, WorkDir: m.WorkDir, Theme: m.Theme,
		Status: m.Status, DesignPath: m.DesignPath, DeckPath: m.DeckPath,
		DeckRevision: m.DeckRevision, DesignRevision: m.DesignRevision,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type threadPO struct {
	ID          string `gorm:"column:id;primaryKey"`
	ProjectID   string `gorm:"column:project_id"`
	Title       string `gorm:"column:title"`
	HistoryPath string `gorm:"column:history_path"`
	Status      string `gorm:"column:status"`
	CreatedAt   int64  `gorm:"column:created_at"`
	UpdatedAt   int64  `gorm:"column:updated_at"`
}

func (threadPO) TableName() string { return "threads" }

func (t threadPO) toModel() model.Thread {
	return model.Thread{
		ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, HistoryPath: t.HistoryPath,
		Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func threadToPO(m model.Thread) threadPO {
	return threadPO{
		ID: m.ID, ProjectID: m.ProjectID, Title: m.Title, HistoryPath: m.HistoryPath,
		Status: m.Status, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type runPO struct {
	ID                string `gorm:"column:id;primaryKey"`
	ThreadID          string `gorm:"column:thread_id"`
	ProjectID         string `gorm:"column:project_id"`
	TargetArtifact    string `gorm:"column:target_artifact"`
	TargetLevel       string `gorm:"column:target_level"`
	TargetSlideID     string `gorm:"column:target_slide_id"`
	InteractionIntent string `gorm:"column:interaction_intent"`
	WorkSpecJSON      string `gorm:"column:work_spec_json"`
	Status            string `gorm:"column:status"`
	CreatedAt         int64  `gorm:"column:created_at"`
	UpdatedAt         int64  `gorm:"column:updated_at"`
}

func (runPO) TableName() string { return "runs" }

func (r runPO) toModel() model.Run {
	var spec model.WorkSpec
	_ = json.Unmarshal([]byte(r.WorkSpecJSON), &spec)
	return model.Run{
		ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID,
		WorkSpec: spec, Status: model.RunStatus(r.Status), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func runToPO(m model.Run) runPO {
	raw, _ := json.Marshal(m.WorkSpec)
	return runPO{
		ID: m.ID, ThreadID: m.ThreadID, ProjectID: m.ProjectID,
		TargetArtifact: string(m.WorkSpec.Target.Artifact),
		TargetLevel:    string(m.WorkSpec.Target.Level), TargetSlideID: m.WorkSpec.Target.SlideID,
		InteractionIntent: string(m.WorkSpec.Interaction.Intent),
		WorkSpecJSON:      string(raw),
		Status:            string(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type runEventPO struct {
	RunID     string `gorm:"column:run_id;primaryKey"`
	Seq       int64  `gorm:"column:seq;primaryKey"`
	Type      string `gorm:"column:type"`
	Payload   string `gorm:"column:payload"`
	CreatedAt int64  `gorm:"column:created_at"`
}

func (runEventPO) TableName() string { return "run_events" }

func (e runEventPO) toModel() model.Event {
	return model.Event{
		RunID: e.RunID, Seq: e.Seq, Type: model.EventType(e.Type),
		Payload: e.Payload, CreatedAt: e.CreatedAt,
	}
}

func eventToPO(m model.Event) runEventPO {
	return runEventPO{
		RunID: m.RunID, Seq: m.Seq, Type: string(m.Type),
		Payload: m.Payload, CreatedAt: m.CreatedAt,
	}
}

type runContextPO struct {
	RunID           string `gorm:"column:run_id;primaryKey"`
	ContextID       string `gorm:"column:context_id;uniqueIndex"`
	Profile         string `gorm:"column:profile"`
	PackHash        string `gorm:"column:pack_hash"`
	EstimatedTokens int    `gorm:"column:estimated_tokens"`
	BudgetTokens    int    `gorm:"column:budget_tokens"`
	ManifestJSON    string `gorm:"column:manifest_json"`
	CreatedAt       int64  `gorm:"column:created_at"`
}

func (runContextPO) TableName() string { return "run_contexts" }

func contextToPO(m model.RunContext) runContextPO {
	return runContextPO{RunID: m.RunID, ContextID: m.ContextID, Profile: m.Profile, PackHash: m.PackHash,
		EstimatedTokens: m.EstimatedTokens, BudgetTokens: m.BudgetTokens, ManifestJSON: m.ManifestJSON, CreatedAt: m.CreatedAt}
}

func (p runContextPO) toModel() model.RunContext {
	return model.RunContext{RunID: p.RunID, ContextID: p.ContextID, Profile: p.Profile, PackHash: p.PackHash,
		EstimatedTokens: p.EstimatedTokens, BudgetTokens: p.BudgetTokens, ManifestJSON: p.ManifestJSON, CreatedAt: p.CreatedAt}
}

type slidePO struct {
	ID                      string `gorm:"column:id;primaryKey"`
	ProjectID               string `gorm:"column:project_id"`
	Position                int    `gorm:"column:position"`
	Layout                  string `gorm:"column:layout"`
	Title                   string `gorm:"column:title"`
	JSONPath                string `gorm:"column:json_path"`
	HTMLPath                string `gorm:"column:html_path"`
	CurrentVersion          int    `gorm:"column:current_version"`
	BlueprintRevision       int    `gorm:"column:blueprint_revision"`
	PresentationRevision    int    `gorm:"column:presentation_revision"`
	SourceDeckRevision      int    `gorm:"column:source_deck_revision"`
	SourceBlueprintRevision int    `gorm:"column:source_blueprint_revision"`
	SourceDesignRevision    int    `gorm:"column:source_design_revision"`
	LastExportAt            *int64 `gorm:"column:last_export_at"`
}

func (slidePO) TableName() string { return "slides" }

func (s slidePO) toModel() model.Slide {
	return model.Slide{
		ID: s.ID, ProjectID: s.ProjectID, Position: s.Position, Layout: s.Layout, Title: s.Title,
		JSONPath: s.JSONPath, HTMLPath: s.HTMLPath, CurrentVersion: s.CurrentVersion,
		BlueprintRevision: s.BlueprintRevision, PresentationRevision: s.PresentationRevision,
		SourceDeckRevision: s.SourceDeckRevision, SourceBlueprintRevision: s.SourceBlueprintRevision,
		SourceDesignRevision: s.SourceDesignRevision, LastExportAt: s.LastExportAt,
	}
}

func slideToPO(m model.Slide) slidePO {
	return slidePO{
		ID: m.ID, ProjectID: m.ProjectID, Position: m.Position, Layout: m.Layout, Title: m.Title,
		JSONPath: m.JSONPath, HTMLPath: m.HTMLPath, CurrentVersion: m.CurrentVersion,
		BlueprintRevision: m.BlueprintRevision, PresentationRevision: m.PresentationRevision,
		SourceDeckRevision: m.SourceDeckRevision, SourceBlueprintRevision: m.SourceBlueprintRevision,
		SourceDesignRevision: m.SourceDesignRevision, LastExportAt: m.LastExportAt,
	}
}

type versionPO struct {
	ID           string  `gorm:"column:id;primaryKey"`
	TargetType   string  `gorm:"column:target_type"`
	TargetID     string  `gorm:"column:target_id"`
	VersionNo    int     `gorm:"column:version_no"`
	SnapshotPath string  `gorm:"column:snapshot_path"`
	RunID        *string `gorm:"column:run_id"`
	CreatedAt    int64   `gorm:"column:created_at"`
}

func (versionPO) TableName() string { return "versions" }

func (v versionPO) toModel() model.Version {
	runID := ""
	if v.RunID != nil {
		runID = *v.RunID
	}
	return model.Version{
		ID: v.ID, TargetType: v.TargetType, TargetID: v.TargetID, VersionNo: v.VersionNo,
		SnapshotPath: v.SnapshotPath, RunID: runID, CreatedAt: v.CreatedAt,
	}
}

func versionToPO(m model.Version) versionPO {
	var runID *string
	if m.RunID != "" {
		runID = &m.RunID
	}
	return versionPO{
		ID: m.ID, TargetType: m.TargetType, TargetID: m.TargetID, VersionNo: m.VersionNo,
		SnapshotPath: m.SnapshotPath, RunID: runID, CreatedAt: m.CreatedAt,
	}
}

type assetPO struct {
	ID           string `gorm:"column:id;primaryKey"`
	Name         string `gorm:"column:name"`
	Kind         string `gorm:"column:kind"`
	Version      string `gorm:"column:version"`
	Source       string `gorm:"column:source"`
	Description  string `gorm:"column:description"`
	Tags         string `gorm:"column:tags"` // JSON 数组文本
	ManifestPath string `gorm:"column:manifest_path"`
	Dir          string `gorm:"column:dir"`
	CreatedAt    int64  `gorm:"column:created_at"`
	UpdatedAt    int64  `gorm:"column:updated_at"`
}

func (assetPO) TableName() string { return "assets" }

func (a assetPO) toModel() model.Asset {
	var tags []string
	if a.Tags != "" {
		_ = json.Unmarshal([]byte(a.Tags), &tags)
	}
	return model.Asset{
		ID: a.ID, Name: a.Name, Kind: a.Kind, Version: a.Version, Source: a.Source,
		Description: a.Description, Tags: tags, ManifestPath: a.ManifestPath, Dir: a.Dir,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func assetToPO(m model.Asset) assetPO {
	tags := "[]"
	if len(m.Tags) > 0 {
		if raw, err := json.Marshal(m.Tags); err == nil {
			tags = string(raw)
		}
	}
	return assetPO{
		ID: m.ID, Name: m.Name, Kind: m.Kind, Version: m.Version, Source: m.Source,
		Description: m.Description, Tags: tags, ManifestPath: m.ManifestPath, Dir: m.Dir,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}
