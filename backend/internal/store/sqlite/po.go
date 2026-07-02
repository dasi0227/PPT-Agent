package sqlite

import "github.com/dasi0227/PPT-Agent/backend/internal/model"

// 持久化对象（PO）：GORM tag 仅出现在本包（ARCH-BACKEND-006）。PO↔model 在 store 边界互转。

type projectPO struct {
	ID         string `gorm:"column:id;primaryKey"`
	Title      string `gorm:"column:title"`
	WorkDir    string `gorm:"column:work_dir"`
	Theme      string `gorm:"column:theme"`
	Status     string `gorm:"column:status"`
	DesignPath string `gorm:"column:design_path"`
	CreatedAt  int64  `gorm:"column:created_at"`
	UpdatedAt  int64  `gorm:"column:updated_at"`
}

func (projectPO) TableName() string { return "projects" }

func (p projectPO) toModel() model.Project {
	return model.Project{
		ID: p.ID, Title: p.Title, WorkDir: p.WorkDir, Theme: p.Theme,
		Status: p.Status, DesignPath: p.DesignPath, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func projectToPO(m model.Project) projectPO {
	return projectPO{
		ID: m.ID, Title: m.Title, WorkDir: m.WorkDir, Theme: m.Theme,
		Status: m.Status, DesignPath: m.DesignPath, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
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
	ID        string `gorm:"column:id;primaryKey"`
	ThreadID  string `gorm:"column:thread_id"`
	ProjectID string `gorm:"column:project_id"`
	Kind      string `gorm:"column:kind"`
	Scope     string `gorm:"column:scope"`
	PageIndex *int   `gorm:"column:page_index"`
	Mode      string `gorm:"column:mode"`
	Command   string `gorm:"column:command"`
	Status    string `gorm:"column:status"`
	CreatedAt int64  `gorm:"column:created_at"`
	UpdatedAt int64  `gorm:"column:updated_at"`
}

func (runPO) TableName() string { return "runs" }

func (r runPO) toModel() model.Run {
	return model.Run{
		ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID, Kind: model.Kind(r.Kind),
		Scope: model.Scope(r.Scope), PageIndex: r.PageIndex, Mode: model.Mode(r.Mode),
		Command: r.Command, Status: model.RunStatus(r.Status), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func runToPO(m model.Run) runPO {
	return runPO{
		ID: m.ID, ThreadID: m.ThreadID, ProjectID: m.ProjectID, Kind: string(m.Kind),
		Scope: string(m.Scope), PageIndex: m.PageIndex, Mode: string(m.Mode),
		Command: m.Command, Status: string(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
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

type slidePO struct {
	ID             string `gorm:"column:id;primaryKey"`
	ProjectID      string `gorm:"column:project_id"`
	Idx            int    `gorm:"column:idx"`
	Layout         string `gorm:"column:layout"`
	Title          string `gorm:"column:title"`
	JSONPath       string `gorm:"column:json_path"`
	HTMLPath       string `gorm:"column:html_path"`
	CurrentVersion int    `gorm:"column:current_version"`
	LastExportAt   *int64 `gorm:"column:last_export_at"`
}

func (slidePO) TableName() string { return "slides" }

func (s slidePO) toModel() model.Slide {
	return model.Slide{
		ID: s.ID, ProjectID: s.ProjectID, Idx: s.Idx, Layout: s.Layout, Title: s.Title,
		JSONPath: s.JSONPath, HTMLPath: s.HTMLPath, CurrentVersion: s.CurrentVersion, LastExportAt: s.LastExportAt,
	}
}

func slideToPO(m model.Slide) slidePO {
	return slidePO{
		ID: m.ID, ProjectID: m.ProjectID, Idx: m.Idx, Layout: m.Layout, Title: m.Title,
		JSONPath: m.JSONPath, HTMLPath: m.HTMLPath, CurrentVersion: m.CurrentVersion, LastExportAt: m.LastExportAt,
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
