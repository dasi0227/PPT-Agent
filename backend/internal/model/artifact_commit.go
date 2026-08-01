package model

type ArtifactCommit struct {
	ProjectID       string
	DeckRevision    int
	DesignRevision  int
	Slides          []Slide
	DeletedSlideIDs []string
	Versions        []Version
}
