package spec

import "github.com/dasi0227/PPT-Agent/backend/internal/designsystem"

type SlideRole string

const (
	SlideRoleCover      SlideRole = "cover"
	SlideRoleAgenda     SlideRole = "agenda"
	SlideRoleContext    SlideRole = "context"
	SlideRoleContent    SlideRole = "content"
	SlideRoleDefinition SlideRole = "definition"
	SlideRoleEvidence   SlideRole = "evidence"
	SlideRoleComparison SlideRole = "comparison"
	SlideRoleExample    SlideRole = "example"
	SlideRoleHowTo      SlideRole = "how-to"
	SlideRoleTransition SlideRole = "transition"
	SlideRoleSummary    SlideRole = "summary"
	SlideRoleConclusion SlideRole = "conclusion"
)

func SlideRoleValues() []SlideRole {
	return []SlideRole{
		SlideRoleCover,
		SlideRoleAgenda,
		SlideRoleContext,
		SlideRoleContent,
		SlideRoleDefinition,
		SlideRoleEvidence,
		SlideRoleComparison,
		SlideRoleExample,
		SlideRoleHowTo,
		SlideRoleTransition,
		SlideRoleSummary,
		SlideRoleConclusion,
	}
}

type Manifest struct {
	Title        string   `json:"title"`
	Goal         string   `json:"goal"`
	Audience     string   `json:"audience"`
	Language     string   `json:"language"`
	Requirements []string `json:"requirements"`
	Prohibitions []string `json:"prohibitions"`
}

type Outline struct {
	Sections []Section `json:"sections"`
}
type Section struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Purpose     string       `json:"purpose"`
	Slides      []SlideNode  `json:"slides"`
	Subsections []Subsection `json:"subsections"`
}
type Subsection struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Purpose string      `json:"purpose"`
	Slides  []SlideNode `json:"slides"`
}
type SlideNode struct {
	SlideID string    `json:"slide_id"`
	Title   string    `json:"title"`
	Role    SlideRole `json:"role"`
}

type SlideSpec struct {
	KeyMessage string    `json:"key_message"`
	Elements   []Element `json:"elements"`
	Layout     string    `json:"layout,omitempty"`
}
type Element struct {
	Type   string `json:"type"`
	Intent string `json:"intent"`
}

type Design struct {
	Direction         string      `json:"direction"`
	LayoutPreferences []string    `json:"layout_preferences"`
	Decorations       Decorations `json:"decorations"`
}
type Decorations struct {
	PageNumber   string `json:"page_number"`
	DeckTitle    string `json:"deck_title"`
	SectionTitle string `json:"section_title"`
	KeyMessage   string `json:"key_message"`
}

func DefaultDecorations() Decorations {
	return Decorations{
		PageNumber:   "bottom-right",
		DeckTitle:    "none",
		SectionTitle: "top-left",
		KeyMessage:   "none",
	}
}

type RuntimeFrameContext struct {
	KeyMessage  string                   `json:"key_message"`
	Appearance  *designsystem.Appearance `json:"appearance"`
	SlideID     string                   `json:"slide_id"`
	Canvas      RuntimeCanvas            `json:"canvas"`
	ThemeID     string                   `json:"theme_id"`
	DeckTitle   string                   `json:"deck_title"`
	Ordinal     int                      `json:"ordinal"`
	Total       int                      `json:"total"`
	Role        string                   `json:"role"`
	Section     RuntimeFrameAncestor     `json:"section"`
	Subsection  *RuntimeFrameAncestor    `json:"subsection,omitempty"`
	Decorations Decorations              `json:"decorations"`
}
type RuntimeFrameAncestor struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Index int    `json:"index"`
}

type ProjectContentSnapshot struct {
	SceneRevision int64                    `json:"scene_revision"`
	ProjectID     string                   `json:"project_id"`
	Theme         string                   `json:"theme"`
	Appearance    *designsystem.Appearance `json:"appearance"`
	ThemeError    string                   `json:"theme_error,omitempty"`
	Hashes        map[string]string        `json:"hashes"`
	Manifest      Manifest                 `json:"manifest"`
	Outline       Outline                  `json:"outline"`
	Design        Design                   `json:"design"`
	SlidesByID    map[string]SlideContent  `json:"slides_by_id"`
}
type SlideContent struct {
	SpecState string     `json:"spec_state"`
	Spec      *SlideSpec `json:"spec"`
	HTMLState string     `json:"html_state"`
	HTMLHash  string     `json:"html_hash"`
}
