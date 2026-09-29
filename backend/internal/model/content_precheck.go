package model

// ContentPrecheck describes one saved page version, never visual acceptance.
type ContentPrecheck struct {
	AssessmentID string                  `json:"assessment_id"`
	SlideID      string                  `json:"slide_id"`
	ContentHash  string                  `json:"content_hash"`
	MaterialHash string                  `json:"material_hash"`
	Status       string                  `json:"status"`
	Reason       string                  `json:"reason,omitempty"`
	Model        string                  `json:"model,omitempty"`
	Rubric       string                  `json:"rubric"`
	Scores       map[string]ContentScore `json:"scores,omitempty"`
}
type ContentScore struct {
	Score         float64            `json:"score"`
	MaxScore      int                `json:"max_score"`
	Legend        map[string]any     `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type ContentPrecheckedPayload struct {
	PublicEventBase
	CallID          string            `json:"call_id"`
	ContentPrecheck []ContentPrecheck `json:"content_precheck"`
}
