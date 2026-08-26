package model

// Slide is one page's runtime identity and version pointer. Authoring content,
// hierarchy and order live only in project files.
type Slide struct {
	ID             string
	ProjectID      string
	CurrentVersion int
	LastExportAt   *int64
}

// Version 是一次可回滚快照的登记（DATA-VERSION）。
type Version struct {
	ID           string
	TargetType   string // outline | slide_spec | slide_html | design | asset
	TargetID     string
	VersionNo    int
	SnapshotPath string
	RunID        string
	CreatedAt    int64
}
