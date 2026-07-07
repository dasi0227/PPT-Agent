package model

// Slide 是一页 slide 的元数据（大文本 slide-json/html 落文件系统，此处仅索引）。
type Slide struct {
	ID             string
	ProjectID      string
	Idx            int
	Layout         string
	Title          string
	JSONPath       string
	HTMLPath       string
	CurrentVersion int
	Order          int
	OutlineDirty   bool
	LastExportAt   *int64
}

// Version 是一次可回滚快照的登记（DATA-VERSION）。
type Version struct {
	ID           string
	TargetType   string // slide | project | design | asset
	TargetID     string
	VersionNo    int
	SnapshotPath string
	RunID        string
	CreatedAt    int64
}
