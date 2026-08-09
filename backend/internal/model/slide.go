package model

// Slide is one page's runtime metadata. Content-derived fields (Position,
// Title, Layout, paths and revisions) are no longer stored in the database.
// They are projected from outline.json, spec.json and materialization.json by
// read-facing services so the API shape stays stable.
type Slide struct {
	ID                    string
	ProjectID             string
	Position              int
	Layout                string
	Title                 string
	SpecPath              string
	HTMLPath              string
	CurrentVersion        int
	SpecRevision          int
	HTMLRevision          int
	SourceOutlineRevision int
	SourceSpecRevision    int
	SourceDesignRevision  int
	LastExportAt          *int64
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
