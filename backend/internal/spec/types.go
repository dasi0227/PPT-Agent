package spec

const SchemaVersion = "3.0"

type Outline struct {
	SchemaVersion string    `json:"version"`
	Revision      int       `json:"revision"`
	ProjectID     string    `json:"project_id"`
	Title         string    `json:"title"`
	Goal          string    `json:"goal"`
	Audience      string    `json:"audience"`
	Language      string    `json:"language"`
	Positioning   string    `json:"positioning,omitempty"`
	Requirements  []string  `json:"requirements"`
	Prohibitions  []string  `json:"prohibitions"`
	Sections      []Section `json:"sections"`
	SlideOrder    []string  `json:"slide_order"`
	CreatedAt     int64     `json:"created_at"`
	UpdatedAt     int64     `json:"updated_at"`
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
	SchemaVersion string    `json:"version"`
	Revision      int       `json:"revision"`
	ProjectID     string    `json:"project_id"`
	SlideID       string    `json:"slide_id"`
	SectionID     string    `json:"section_id"`
	SubsectionID  string    `json:"subsection_id,omitempty"`
	Role          string    `json:"role"`
	Title         string    `json:"title"`
	KeyMessage    string    `json:"key_message"`
	Elements      []Element `json:"elements"`
	Layout        string    `json:"layout,omitempty"`
	CreatedAt     int64     `json:"created_at"`
	UpdatedAt     int64     `json:"updated_at"`
}

type Element struct {
	Type   string `json:"type"`
	Intent string `json:"intent"`
}

type Design struct {
	SchemaVersion string       `json:"version"`
	Revision      int          `json:"revision"`
	ProjectID     string       `json:"project_id"`
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

type MaterializationRecord struct {
	SchemaVersion string                  `json:"version"`
	Artifact      MaterializationArtifact `json:"artifact"`
	Source        MaterializationSource   `json:"source"`
	RenderedAt    int64                   `json:"rendered_at"`
}

type MaterializationArtifact struct {
	Revision int    `json:"revision"`
	Hash     string `json:"hash"`
}

type MaterializationSource struct {
	Outline int    `json:"outline"`
	Spec    int    `json:"spec"`
	Design  int    `json:"design"`
	Hash    string `json:"hash"`
}
