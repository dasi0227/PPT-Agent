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

type PresentationManifestContext struct {
	Manifest pptspec.Manifest `json:"manifest"`
	State    string           `json:"-"`
}

func (c PresentationManifestContext) MarshalJSON() ([]byte, error) {
	if c.State == "missing" {
		return []byte(`{"status":"missing"}`), nil
	}
	return json.Marshal(struct {
		Manifest pptspec.Manifest `json:"manifest"`
	}{Manifest: c.Manifest})
}

type SlideSummary struct {
	ID         string `json:"id"`
	Ordinal    int    `json:"ordinal"`
	Section    string `json:"section"`
	Subsection string `json:"subsection,omitempty"`
	Purpose    string `json:"purpose,omitempty"`
	Approach   string `json:"approach,omitempty"`
	Title      string `json:"title"`
	Core       string `json:"core,omitempty"`
	State      string `json:"html_state,omitempty"`
}

type DesignContext struct {
	Design *pptspec.Design `json:"design,omitempty"`
	State  string          `json:"-"`
}

func (c DesignContext) MarshalJSON() ([]byte, error) {
	if c.State == "missing" {
		return []byte(`{"status":"missing"}`), nil
	}
	return json.Marshal(struct {
		Design *pptspec.Design `json:"design,omitempty"`
	}{Design: c.Design})
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
	Profile              ProfileID                   `json:"profile"`
	Command              model.RunCommand            `json:"run_command"`
	Project              ProjectContext              `json:"project"`
	PresentationManifest PresentationManifestContext `json:"presentation_manifest"`
	Outline              OutlineContext              `json:"outline"`
	Design               DesignContext               `json:"design"`
	Components           []ComponentCandidate        `json:"components"`
	Skills               []SkillCandidate            `json:"skills"`
	Manifest             ContextManifest             `json:"manifest"`
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
