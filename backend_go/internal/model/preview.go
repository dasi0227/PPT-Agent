package model

import (
	"errors"
	"math"
)

const PreviewRubricVersion = "preview-v4"

const (
	ContentCoverage      = "content_coverage"
	ExpressionClarity    = "expression_clarity"
	RequirementAdherence = "requirement_adherence"
)

// PreviewRubrics is the single source for scoring instructions and levels.
// Each call returns independent values.
func PreviewRubrics() [3]PreviewDimensionRubric {
	return [3]PreviewDimensionRubric{
		{
			Dimension:    ContentCoverage,
			Instructions: "Score how completely the slide fulfills its intended content role.",
			Criteria: [4]string{
				"Content is absent or unrelated to the slide's purpose.",
				"The central message or major required material is missing.",
				"The central message and most required material are present, with limited omissions.",
				"The central message and required supporting material are complete.",
			},
		},
		{
			Dimension:    ExpressionClarity,
			Instructions: "Score the clarity of the slide's message, organization, and wording.",
			Criteria: [4]string{
				"No understandable message or meaningful organization.",
				"Confusing structure or wording makes the message difficult to identify.",
				"The main message is clear, with minor ambiguity or unnecessary complexity.",
				"The message, organization, and wording are consistently clear.",
			},
		},
		{
			Dimension:    RequirementAdherence,
			Instructions: "Score compliance with the slide-applicable requirements in meta, constraints, and spec.",
			Criteria: [4]string{
				"A key requirement or restriction is contradicted.",
				"Material deviations from applicable requirements.",
				"Core requirements are met, with minor deviations.",
				"All applicable requirements and restrictions are met.",
			},
		},
	}
}

type PreviewDimensionRubric struct {
	Dimension    string
	Instructions string
	Criteria     [4]string
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
	if !oneOf(p.Judges.Mismatch, "meta_mismatch", "content_mismatch", "spec_mismatch", "visual_mismatch", "none") || !oneOf(p.Judges.Issue, "expression_issue", "layout_issue", "visual_issue", "none") {
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
