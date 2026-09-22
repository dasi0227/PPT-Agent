package model

// Slide is one page's runtime identity. Authoring content,
// hierarchy and order live only in project files.
type Slide struct {
	ID           string
	ProjectID    string
	LastExportAt *int64
}
