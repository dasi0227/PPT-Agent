package contextengine

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const SchemaVersion = "2.0"

type ProfileID string

const (
	ProfileSpecDeck  ProfileID = "spec/deck"
	ProfileSpecSlide ProfileID = "spec/slide"
	ProfilePPTDeck   ProfileID = "ppt/deck"
	ProfilePPTSlide  ProfileID = "ppt/slide"
)

type SegmentKind string

const (
	SegmentPolicy               SegmentKind = "policy"
	SegmentRunCommand           SegmentKind = "run_command"
	SegmentPresentationManifest SegmentKind = "presentation_manifest"
	SegmentOutline              SegmentKind = "outline"
	SegmentTarget               SegmentKind = "target_artifact"
	SegmentRelated              SegmentKind = "related_slides"
	SegmentDesign               SegmentKind = "design"
	SegmentTheme                SegmentKind = "theme"
	SegmentSlideHTML            SegmentKind = "slide_html"
	SegmentComponents           SegmentKind = "components"
	SegmentMemory               SegmentKind = "thread_memory"
	SegmentRecentTurns          SegmentKind = "recent_turns"
)

type DetailLevel string

const (
	DetailSummary   DetailLevel = "summary"
	DetailStructure DetailLevel = "structure"
	DetailFull      DetailLevel = "full"
)

type TokenBudget struct {
	ContextWindow int                 `json:"context_window"`
	InputLimit    int                 `json:"input_limit"`
	OutputReserve int                 `json:"output_reserve"`
	SegmentCaps   map[SegmentKind]int `json:"segment_caps"`
}

func DefaultBudget() TokenBudget {
	return TokenBudget{ContextWindow: 32768, InputLimit: 20000, OutputReserve: 8000, SegmentCaps: map[SegmentKind]int{
		SegmentPolicy: 3000, SegmentRunCommand: 1200, SegmentPresentationManifest: 1600, SegmentOutline: 3000, SegmentTarget: 6000,
		SegmentRelated: 2400, SegmentDesign: 3000, SegmentTheme: 2400, SegmentSlideHTML: 6000,
		SegmentComponents: 1800, SegmentMemory: 2000, SegmentRecentTurns: 1200,
	}}
}

type ContextRequest struct {
	RunID     string
	ThreadID  string
	ProjectID string
	Command   model.RunCommand
	Budget    TokenBudget
}

type ProjectContext struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type OutlineContext struct {
	Outline   pptspec.Outline `json:"outline"`
	Summaries []SlideSummary  `json:"slide_summaries"`
}

type PresentationManifestContext struct {
	Manifest pptspec.Manifest `json:"manifest"`
}

type SlideSummary struct {
	ID         string `json:"id"`
	Ordinal    int    `json:"ordinal"`
	Section    string `json:"section"`
	Subsection string `json:"subsection,omitempty"`
	Role       string `json:"role"`
	Title      string `json:"title"`
	KeyMessage string `json:"key_message"`
	State      string `json:"materialization_state,omitempty"`
}

type TargetContext struct {
	Artifact         model.Artifact           `json:"artifact"`
	Level            model.ScopeLevel         `json:"level"`
	SlideSpec        *pptspec.SlideSpec       `json:"slide_spec,omitempty"`
	Materialization  *pptspec.Materialization `json:"materialization,omitempty"`
	SlideHTMLSummary *HTMLSummary             `json:"slide_html_summary,omitempty"`
	SlideHTML        string                   `json:"slide_html,omitempty"`
	SlideHTMLRef     *ContextRef              `json:"slide_html_ref,omitempty"`
}

type DesignContext struct {
	Design *pptspec.Design `json:"design,omitempty"`
}

type ThemeToken struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ThemeContext struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	Tokens           []ThemeToken `json:"tokens"`
	AllowedSelectors []string     `json:"allowed_selectors"`
	Source           string       `json:"source"`
	Trust            string       `json:"trust"`
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

type RecentTurn struct {
	Turn  string `json:"turn"`
	Type  string `json:"type"`
	Text  string `json:"text"`
	RunID string `json:"run_id,omitempty"`
}

type RevisionRefs struct {
	Manifest     int            `json:"manifest"`
	Outline      int            `json:"outline"`
	Design       int            `json:"design"`
	SlideSpecs   map[string]int `json:"slide_specs"`
	SlideHTML    map[string]int `json:"slide_html"`
	ThreadMemory int            `json:"thread_memory"`
}

type ContextPack struct {
	SchemaVersion        string                      `json:"schema_version"`
	Profile              ProfileID                   `json:"profile"`
	Command              model.RunCommand            `json:"run_command"`
	Project              ProjectContext              `json:"project"`
	PresentationManifest PresentationManifestContext `json:"presentation_manifest"`
	Outline              OutlineContext              `json:"outline"`
	Target               TargetContext               `json:"target"`
	RelatedSlides        []SlideSummary              `json:"related_slides"`
	Design               DesignContext               `json:"design"`
	Theme                *ThemeContext               `json:"theme,omitempty"`
	SlideHTML            SlideHTMLContext            `json:"slide_html"`
	Components           []ComponentCandidate        `json:"components"`
	Memory               ThreadMemory                `json:"memory"`
	RecentTurns          []RecentTurn                `json:"recent_turns"`
	Revisions            RevisionRefs                `json:"revisions"`
	Manifest             ContextManifest             `json:"manifest"`
	RefResolver          *ContextRefResolver         `json:"-"`
}

type ContextSegment struct {
	ID              string      `json:"id"`
	Kind            SegmentKind `json:"kind"`
	SourceRef       string      `json:"source_ref"`
	Revision        int         `json:"revision"`
	ContentHash     string      `json:"content_hash"`
	EstimatedTokens int         `json:"estimated_tokens"`
	Priority        int         `json:"priority"`
	SelectionReason string      `json:"selection_reason"`
	DetailLevel     DetailLevel `json:"detail_level"`
	Required        bool        `json:"required"`
}

type DroppedSegment struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type ContextManifest struct {
	ContextID       string           `json:"context_id"`
	RunID           string           `json:"run_id"`
	ThreadID        string           `json:"thread_id"`
	ProjectID       string           `json:"project_id"`
	Profile         ProfileID        `json:"profile"`
	ReadOnly        bool             `json:"read_only"`
	EstimatedTokens int              `json:"estimated_tokens"`
	BudgetTokens    int              `json:"budget_tokens"`
	OutputReserve   int              `json:"output_reserve"`
	PackHash        string           `json:"pack_hash"`
	Segments        []ContextSegment `json:"segments"`
	Refs            []ContextRef     `json:"refs"`
	Dropped         []DroppedSegment `json:"dropped"`
	Warnings        []string         `json:"warnings"`
}

type candidate struct {
	segment ContextSegment
	value   any
	apply   func(*ContextPack)
	drop    func(*ContextPack)
}

func stableJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
