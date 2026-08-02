package model

type ArtifactCommit struct {
	ProjectID       string
	OutlineRevision int
	DesignRevision  int
	Slides          []Slide
	DeletedSlideIDs []string
	Versions        []Version
}
