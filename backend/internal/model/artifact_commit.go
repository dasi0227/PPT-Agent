package model

type ArtifactCommit struct {
	ProjectID       string
	Slides          []Slide
	DeletedSlideIDs []string
	Versions        []Version
}
