package blueprint

const SchemaVersion = "2.0"

type Deck struct {
	SchemaVersion string    `json:"schema_version"`
	Revision      int       `json:"revision"`
	ProjectID     string    `json:"project_id"`
	Title         string    `json:"title"`
	Goal          string    `json:"goal"`
	Audience      string    `json:"audience"`
	Language      string    `json:"language"`
	CoreThesis    string    `json:"core_thesis"`
	NarrativeArc  string    `json:"narrative_arc"`
	Sections      []Section `json:"sections"`
	SlideOrder    []string  `json:"slide_order"`
	CreatedAt     int64     `json:"created_at"`
	UpdatedAt     int64     `json:"updated_at"`
}

type Section struct {
	ID          string       `json:"id"`
	Number      string       `json:"number"`
	Title       string       `json:"title"`
	Subsections []Subsection `json:"subsections"`
}

type Subsection struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Title  string `json:"title"`
}

type Slide struct {
	SchemaVersion string       `json:"schema_version"`
	Revision      int          `json:"revision"`
	SlideID       string       `json:"slide_id"`
	SectionID     string       `json:"section_id"`
	SubsectionID  string       `json:"subsection_id,omitempty"`
	Role          string       `json:"role"`
	Title         string       `json:"title"`
	KeyMessage    string       `json:"key_message"`
	Content       Content      `json:"content"`
	VisualIntent  VisualIntent `json:"visual_intent"`
	SpeakerNotes  string       `json:"speaker_notes"`
	CreatedAt     int64        `json:"created_at"`
	UpdatedAt     int64        `json:"updated_at"`
}

type Content struct {
	Summary string   `json:"summary"`
	Points  []string `json:"points"`
}

type VisualIntent struct {
	Archetype    string   `json:"archetype"`
	Description  string   `json:"description"`
	AssetQueries []string `json:"asset_queries"`
}

type DesignSpec struct {
	SchemaVersion string         `json:"schema_version"`
	Revision      int            `json:"revision"`
	Canvas        map[string]any `json:"canvas"`
	Palette       []string       `json:"palette"`
	Typography    map[string]any `json:"typography"`
	Spacing       map[string]any `json:"spacing"`
	Radius        map[string]any `json:"radius"`
	Shadows       map[string]any `json:"shadows"`
	LayoutSystem  map[string]any `json:"layout_system"`
	Signature     string         `json:"signature"`
	Motion        map[string]any `json:"motion"`
}

type ProjectView struct {
	Deck       Deck                       `json:"deck"`
	Slides     map[string]Slide           `json:"slides"`
	DesignSpec DesignSpec                 `json:"design_spec"`
	States     map[string]Materialization `json:"materialization"`
}

type Materialization struct {
	State     string `json:"state"`
	Revisions any    `json:"revisions"`
}
