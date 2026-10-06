package contextengine

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const SchemaVersion = "2.0"

type ProfileID string

const (
	ProfilePPTDeck  ProfileID = "ppt/deck"
	ProfilePPTSlide ProfileID = "ppt/slide"
)

type ContextRequest struct {
	RunID     string
	ThreadID  string
	ProjectID string
	Command   model.RunCommand
}

type ProjectContext struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	ThemeID string `json:"-"`
}

type OutlineContext struct {
	Outline   pptspec.Outline `json:"outline"`
	Summaries []SlideSummary  `json:"slide_summaries"`
}

type PresentationManifestContext struct {
	Manifest pptspec.Manifest `json:"manifest"`
}

type SlideSummary struct {
	ID          string `json:"id"`
	Ordinal     int    `json:"ordinal"`
	Section     string `json:"section"`
	Subsection  string `json:"subsection,omitempty"`
	Purpose     string `json:"purpose,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Title       string `json:"title"`
	Core        string `json:"core"`
	State       string `json:"html_state,omitempty"`
}

type TargetContext struct {
	SlideIDs         []string           `json:"slide_ids"`
	SlideSpec        *pptspec.SlideSpec `json:"slide_spec,omitempty"`
	SlideHTMLSummary *HTMLSummary       `json:"slide_html_summary,omitempty"`
}

type DesignContext struct {
	Design *pptspec.Design `json:"design,omitempty"`
}

type SlideHTMLContext struct {
	Summaries map[string]HTMLSummary `json:"summaries"`
}

type ComponentCandidate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags"`
}

type SkillCandidate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags"`
}

type ContextPack struct {
	SchemaVersion        string                               `json:"schema_version"`
	Profile              ProfileID                            `json:"profile"`
	Command              model.RunCommand                     `json:"run_command"`
	Project              ProjectContext                       `json:"project"`
	PresentationManifest PresentationManifestContext          `json:"presentation_manifest"`
	Outline              OutlineContext                       `json:"outline"`
	Target               TargetContext                        `json:"target"`
	Design               DesignContext                        `json:"design"`
	SlideHTML            SlideHTMLContext                     `json:"slide_html"`
	Components           []ComponentCandidate                 `json:"components"`
	Skills               []SkillCandidate                     `json:"skills"`
	Manifest             ContextManifest                      `json:"manifest"`
	GenerationInputs     map[string]*pptspec.GenerationInputs `json:"-"`
	GenerationBaselines  map[string]*pptspec.GenerationInputs `json:"-"`
}

type ContextManifest struct {
	ContextID string   `json:"context_id"`
	RunID     string   `json:"run_id"`
	ThreadID  string   `json:"thread_id"`
	ProjectID string   `json:"project_id"`
	PackHash  string   `json:"pack_hash"`
	Warnings  []string `json:"warnings"`
}

func stableJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
