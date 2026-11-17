package spec

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

type SlidePurpose string

type Meta struct {
	Title    string `json:"title"`
	Language string `json:"language"`
	Count    string `json:"count"`
	Audience string `json:"audience"`
	Goal     string `json:"goal"`
}

func DefaultMeta(title string) Meta {
	return Meta{Title: title}
}

type Outline struct {
	Sections []Section `json:"sections"`
}
type Section struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Slides      []SlideNode  `json:"slides"`
	Subsections []Subsection `json:"subsections"`
}
type Subsection struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Slides []SlideNode `json:"slides"`
}
type SlideNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type SlideSpec struct {
	Purpose  SlidePurpose `json:"purpose"`
	Core     string       `json:"core"`
	Elements []Element    `json:"elements"`
	Layout   string       `json:"layout"`
}
type Element struct {
	Type   string `json:"type"`
	Intent string `json:"intent"`
}

type ConstraintSet struct {
	Requirements []string `json:"requirements"`
	Prohibitions []string `json:"prohibitions"`
}

type Constraints struct {
	Content ConstraintSet `json:"content"`
	Visual  ConstraintSet `json:"visual"`
}

func DefaultConstraints() Constraints {
	return Constraints{
		Content: ConstraintSet{Requirements: []string{}, Prohibitions: []string{}},
		Visual:  ConstraintSet{Requirements: []string{}, Prohibitions: []string{}},
	}
}

type RuntimeFrameContext struct {
	Logo        *decoration.Image        `json:"logo"`
	Appearance  *designsystem.Appearance `json:"appearance"`
	SlideID     string                   `json:"slide_id"`
	Canvas      RuntimeCanvas            `json:"canvas"`
	ThemeID     string                   `json:"theme_id"`
	DeckTitle   string                   `json:"deck_title"`
	Ordinal     int                      `json:"ordinal"`
	Total       int                      `json:"total"`
	Purpose     string                   `json:"purpose,omitempty"`
	Section     RuntimeFrameAncestor     `json:"section"`
	Subsection  *RuntimeFrameAncestor    `json:"subsection,omitempty"`
	Decorations decoration.Positions     `json:"decorations"`
}
type RuntimeFrameAncestor struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Index int    `json:"index"`
}

type ProjectContentSnapshot struct {
	SceneRevision int64                    `json:"scene_revision,string"`
	ProjectID     string                   `json:"project_id"`
	ProjectTitle  string                   `json:"project_title"`
	Theme         string                   `json:"theme"`
	Decorations   decoration.Positions     `json:"decorations"`
	Logo          *decoration.Image        `json:"logo"`
	Appearance    *designsystem.Appearance `json:"appearance"`
	ThemeError    string                   `json:"theme_error,omitempty"`
	Hashes        map[string]string        `json:"hashes"`
	Meta          *Meta                    `json:"meta"`
	Outline       Outline                  `json:"outline"`
	Constraints   *Constraints             `json:"constraints"`
	SlidesByID    map[string]SlideContent  `json:"slides_by_id"`
}
type SlideContent struct {
	SpecState string     `json:"spec_state"`
	Spec      *SlideSpec `json:"spec"`
	HTMLState string     `json:"html_state"`
	HTMLHash  string     `json:"html_hash"`
}
