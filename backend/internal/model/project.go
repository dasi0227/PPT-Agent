package model

// Project is one PPT workspace. OutlineRevision and DesignRevision are
// read-time projections from files; SQLite stores identity and orchestration
// metadata only.
type Project struct {
	ID              string
	Title           string
	WorkDir         string
	Theme           string
	Status          string
	OutlineRevision int
	DesignRevision  int
	LayoutVersion   int
	CreatedAt       int64
	UpdatedAt       int64
}

// Thread 是一条对话线程；Run 挂在其下，提供 project 归属。
type Thread struct {
	ID          string
	ProjectID   string
	Title       string
	HistoryPath string
	Status      string
	CreatedAt   int64
	UpdatedAt   int64
}
