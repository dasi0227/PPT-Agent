package model

import (
	"errors"
	"math"
)

const PreviewRubricVersion = "preview-v1"

const (
	ContentCoverage      = "content_coverage"
	ExpressionClarity    = "expression_clarity"
	RequirementAdherence = "requirement_adherence"
)

// PreviewRubrics supplies both the scoring questions and the descriptions
// in model-visible tool output contracts. Each call returns independent values.
func PreviewRubrics() [3]PreviewDimensionRubric {
	return [3]PreviewDimensionRubric{
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

type PreviewDimensionRubric struct {
	Dimension   string
	Description string
	Criteria    [4]string
}

// Preview is the complete model-visible result. Metadata stays on the assessment.
type Preview struct {
	Scores map[string]float64 `json:"scores"`
	Judges PreviewJudges      `json:"judges"`
}
type PreviewJudges struct {
	Mismatch string `json:"mismatch"`
	Issue    string `json:"issue"`
}

func (p *Preview) Validate() error {
	if p == nil || len(p.Scores) != 3 {
		return errors.New("preview requires all scores")
	}
	for _, rubric := range PreviewRubrics() {
		score, ok := p.Scores[rubric.Dimension]
		if !ok || math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 3 {
			return errors.New("invalid preview score")
		}
	}
	if !oneOf(p.Judges.Mismatch, "manifest_mismatch", "spec_mismatch", "design_mismatch", "none") || !oneOf(p.Judges.Issue, "expression_issue", "layout_issue", "visual_issue", "none") {
		return errors.New("invalid preview judge")
	}
	return nil
}

// PreviewAssessment is lifecycle data for persistence and UI replay, never the
// model-visible tool value. An invalidated assessment has no Result.
type PreviewAssessment struct {
	AssessmentID string   `json:"assessment_id"`
	SlideID      string   `json:"slide_id"`
	MaterialHash string   `json:"material_hash"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason,omitempty"`
	Model        string   `json:"model,omitempty"`
	Rubric       string   `json:"rubric"`
	Result       *Preview `json:"result,omitempty"`
}
type PreviewUpdatedPayload struct {
	PublicEventBase
	SourceRunID string             `json:"source_run_id"`
	CallID      string             `json:"call_id"`
	Preview     *PreviewAssessment `json:"preview"`
}
