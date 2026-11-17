package contextengine

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

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
	ID          string                   `json:"id"`
	Title       string                   `json:"title"`
	ThemeID     string                   `json:"-"`
	Decorations decoration.Positions     `json:"-"`
	Logo        *decoration.LogoMetadata `json:"-"`
}

type OutlineContext struct {
	Outline   pptspec.Outline `json:"outline"`
	Summaries []SlideSummary  `json:"slide_summaries"`
}

type PresentationMetaContext struct {
	Meta  pptspec.Meta `json:"meta"`
	State string       `json:"-"`
}

func (c PresentationMetaContext) MarshalJSON() ([]byte, error) {
	if c.State == "missing" {
		return []byte(`{"status":"missing"}`), nil
	}
	return json.Marshal(struct {
		Meta pptspec.Meta `json:"meta"`
	}{Meta: c.Meta})
}

type SlideSummary struct {
	ID         string `json:"id"`
	Ordinal    int    `json:"ordinal"`
	Section    string `json:"section"`
	Subsection string `json:"subsection,omitempty"`
	Purpose    string `json:"purpose,omitempty"`
	Title      string `json:"title"`
	Core       string `json:"core,omitempty"`
	State      string `json:"html_state,omitempty"`
}

type ConstraintsContext struct {
	Constraints *pptspec.Constraints `json:"constraints,omitempty"`
	State       string               `json:"-"`
}

func (c ConstraintsContext) MarshalJSON() ([]byte, error) {
	if c.State == "missing" {
		return []byte(`{"status":"missing"}`), nil
	}
	return json.Marshal(struct {
		Constraints *pptspec.Constraints `json:"constraints,omitempty"`
	}{Constraints: c.Constraints})
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
	Profile          ProfileID               `json:"profile"`
	Command          model.RunCommand        `json:"run_command"`
	Project          ProjectContext          `json:"project"`
	PresentationMeta PresentationMetaContext `json:"presentation_meta"`
	Outline          OutlineContext          `json:"outline"`
	Constraints      ConstraintsContext      `json:"constraints"`
	Components       []ComponentCandidate    `json:"components"`
	Skills           []SkillCandidate        `json:"skills"`
	Manifest         ContextManifest         `json:"manifest"`

	// The first task input may be a steering message after an independent reply.
	OmitInitialInstruction bool `json:"-"`
}

type ContextManifest struct {
	ContextID string   `json:"context_id"`
	RunID     string   `json:"run_id"`
	ThreadID  string   `json:"thread_id"`
	Warnings  []string `json:"warnings"`
}

func stableJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
