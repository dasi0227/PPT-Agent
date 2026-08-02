package spec

const SchemaVersion = "3.0"

type Outline struct {
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

type SlideSpec struct {
	SchemaVersion         string       `json:"schema_version"`
	Revision              int          `json:"revision"`
	ProjectID             string       `json:"project_id"`
	SlideID               string       `json:"slide_id"`
	SourceOutlineRevision int          `json:"source_outline_revision"`
	SectionID             string       `json:"section_id"`
	SubsectionID          string       `json:"subsection_id,omitempty"`
	Role                  string       `json:"role"`
	Title                 string       `json:"title"`
	KeyMessage            string       `json:"key_message"`
	Content               Content      `json:"content"`
	VisualIntent          VisualIntent `json:"visual_intent"`
	SpeakerNotes          string       `json:"speaker_notes"`
	CreatedAt             int64        `json:"created_at"`
	UpdatedAt             int64        `json:"updated_at"`
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

type Design struct {
	SchemaVersion string         `json:"schema_version"`
	Revision      int            `json:"revision"`
	ProjectID     string         `json:"project_id"`
	Canvas        CanvasSpec     `json:"canvas"`
	Palette       []string       `json:"palette"`
	Typography    TypographySpec `json:"typography"`
	Spacing       SpacingSpec    `json:"spacing"`
	Radius        RadiusSpec     `json:"radius"`
	Shadows       ShadowSpec     `json:"shadows"`
	LayoutSystem  LayoutSystem   `json:"layout_system"`
	Signature     string         `json:"signature"`
	Motion        MotionSpec     `json:"motion"`
	CreatedAt     int64          `json:"created_at"`
	UpdatedAt     int64          `json:"updated_at"`
}

type CanvasSpec struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Ratio  string `json:"ratio"`
}

type FontSpec struct {
	Family string `json:"family"`
	Weight int    `json:"weight,omitempty"`
}

type TypographySpec struct {
	Display FontSpec `json:"display"`
	Body    FontSpec `json:"body"`
	Utility FontSpec `json:"utility"`
}

type SpacingSpec struct {
	Unit int `json:"unit"`
}

type RadiusSpec struct {
	Card int `json:"card"`
}

type ShadowSpec struct {
	Card string `json:"card"`
}

type LayoutSystem struct {
	Grid    string `json:"grid"`
	Rhythm  string `json:"rhythm"`
	Density string `json:"density"`
}

type MotionSpec struct {
	Policy string `json:"policy"`
}

type ProjectView struct {
	Outline    Outline                    `json:"outline"`
	SlideSpecs map[string]SlideSpec       `json:"slide_specs"`
	Design     Design                     `json:"design"`
	States     map[string]Materialization `json:"materialization"`
}

type Materialization struct {
	State     string `json:"state"`
	Revisions any    `json:"revisions"`
}
