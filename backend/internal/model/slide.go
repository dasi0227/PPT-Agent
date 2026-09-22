package model

// Slide is one page's runtime identity and HTML change counter. Authoring content,
// hierarchy and order live only in project files.
type Slide struct {
	ID        string
	ProjectID string
	// CurrentVersion invalidates HTML caches; it does not reference a historical file.
	CurrentVersion int
	LastExportAt   *int64
}
