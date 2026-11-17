package model

type ArtifactCommit struct {
	ProjectID       string
	RunID           string
	OperationID     string
	RequestHash     string
	ToolResultJSON  string
	Slides          []Slide
	DeletedSlideIDs []string
}
