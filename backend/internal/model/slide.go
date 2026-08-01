package model

// Slide 是一页 slide 的元数据（canonical Blueprint/HTML 落文件系统，此处仅索引）。
type Slide struct {
	ID                      string
	ProjectID               string
	Position                int
	Layout                  string
	Title                   string
	JSONPath                string
	HTMLPath                string
	CurrentVersion          int
	BlueprintRevision       int
	PresentationRevision    int
	SourceDeckRevision      int
	SourceBlueprintRevision int
	SourceDesignRevision    int
	LastExportAt            *int64
}

// Version 是一次可回滚快照的登记（DATA-VERSION）。
type Version struct {
	ID           string
	TargetType   string // blueprint_deck | blueprint_slide | presentation_slide | design | asset
	TargetID     string
	VersionNo    int
	SnapshotPath string
	RunID        string
	CreatedAt    int64
}
