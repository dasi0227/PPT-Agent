package spec

const SchemaVersion = "3.0"

type Outline struct {
	SchemaVersion string      `json:"version"`
	Revision      int         `json:"revision"`
	ProjectID     string      `json:"project"`
	Title         string      `json:"title"`
	Goal          string      `json:"goal"`
	Audience      string      `json:"audience"`
	Language      string      `json:"language"`
	Positioning   string      `json:"positioning,omitempty"`
	Constraints   Constraints `json:"constraints"`
	Sections      []Section   `json:"sections"`
	SlideOrder    []string    `json:"slide_order"`
	CreatedAt     int64       `json:"created_at"`
	UpdatedAt     int64       `json:"updated_at"`
}

type Constraints struct {
	MustInclude   []string `json:"must_include"`
	MustAvoid     []string `json:"must_avoid"`
	StyleLimits   []string `json:"style_limits"`
	ContentLimits []string `json:"content_limits"`
}

type Section struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Purpose     string       `json:"purpose"`
	Subsections []Subsection `json:"subsections"`
}

type Subsection struct {
	ID    string `json:"id"`
	Title string `json:"title"`
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
	SchemaVersion string       `json:"version"`
	Revision      int          `json:"revision"`
	ProjectID     string       `json:"project"`
	Theme         string       `json:"theme"`
	Direction     string       `json:"direction"`
	Density       string       `json:"density"`
	Chrome        []ChromeItem `json:"chrome"`
	CreatedAt     int64        `json:"created_at"`
	UpdatedAt     int64        `json:"updated_at"`
}

type ChromeItem struct {
	Type      string `json:"type"`
	Placement string `json:"placement"`
	Style     string `json:"style"`
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
