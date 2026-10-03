package model

const ContentPrecheckRubricVersion = "content-v2"

const (
	ContentCoverage      = "content_coverage"
	ExpressionClarity    = "expression_clarity"
	RequirementAdherence = "requirement_adherence"
)

// ContentPrecheckRubrics supplies both the scoring questions and the descriptions
// in model-visible tool output contracts. Each call returns independent values.
func ContentPrecheckRubrics() [3]ContentDimensionRubric {
	return [3]ContentDimensionRubric{
		{
			Dimension:   ContentCoverage,
			Description: "Whether the current page contains the central message and required supporting material needed to perform its intended role.",
			Criteria: [4]string{
				"The actual page content is absent or unrelated to the page's required purpose.",
				"Some required points appear but the central message or major required material is missing.",
				"The central message and most required material are present, with specific remaining omissions.",
				"The actual content fully performs this page's intended role under the user requirements.",
			},
		},
		{
			Dimension:   ExpressionClarity,
			Description: "Whether the current page's main message, supporting organization and wording are understandable and clear.",
			Criteria: [4]string{
				"The text has no understandable message or meaningful organization.",
				"The intended message can only be guessed because the structure or wording is confusing.",
				"The main message is clear, with limited ambiguity or unnecessary complexity.",
				"The conclusion, supporting organization and wording are consistently clear and understandable.",
			},
		},
		{
			Dimension:   RequirementAdherence,
			Description: "Whether the current page content follows the effective user requirements and restrictions, with original instructions and later corrections taking precedence over editable project resources.",
			Criteria: [4]string{
				"The current content contradicts a key user requirement or restriction.",
				"The current content has material deviations from the effective user requirements.",
				"The current content follows the core requirements with minor deviations.",
				"The current content follows the effective user requirements and restrictions.",
			},
		},
	}
}

type ContentDimensionRubric struct {
	Dimension   string
	Description string
	Criteria    [4]string
}

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
