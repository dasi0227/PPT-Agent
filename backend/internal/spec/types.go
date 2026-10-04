package spec

import "github.com/dasi0227/PPT-Agent/backend/internal/designsystem"

type SlidePurpose string

const (
	SlidePurposeCover        SlidePurpose = "cover"
	SlidePurposeIntroduction SlidePurpose = "introduction"
	SlidePurposeTransition   SlidePurpose = "transition"
	SlidePurposeContent      SlidePurpose = "content"
	SlidePurposeConclusion   SlidePurpose = "conclusion"
	SlidePurposeOther        SlidePurpose = "other"
)

func SlidePurposeValues() []SlidePurpose {
	return []SlidePurpose{
		SlidePurposeCover,
		SlidePurposeIntroduction,
		SlidePurposeTransition,
		SlidePurposeContent,
		SlidePurposeConclusion,
		SlidePurposeOther,
	}
}

type SlideContentType string

const (
	SlideContentTypeExplanation SlideContentType = "explanation"
	SlideContentTypeComparison  SlideContentType = "comparison"
	SlideContentTypeExample     SlideContentType = "example"
	SlideContentTypeGuidance    SlideContentType = "guidance"
	SlideContentTypeOther       SlideContentType = "other"
)

type Manifest struct {
	Title        string   `json:"title"`
	Language     string   `json:"language"`
	Pages        string   `json:"pages"`
	Audience     string   `json:"audience"`
	Goal         string   `json:"goal"`
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
	SlideID string `json:"slide_id"`
	Title   string `json:"title"`
}

type SlideSpec struct {
	Purpose     SlidePurpose     `json:"purpose,omitempty"`
	ContentType SlideContentType `json:"content_type,omitempty"`
	Core        string           `json:"core"`
	Elements    []Element        `json:"elements"`
	Layout      string           `json:"layout,omitempty"`
}
type Element struct {
	Type   string `json:"type"`
	Intent string `json:"intent"`
}

type Design struct {
	Demands     []string    `json:"demands"`
	Decorations Decorations `json:"decorations"`
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
	Purpose     string                   `json:"purpose,omitempty"`
	ContentType string                   `json:"content_type,omitempty"`
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
