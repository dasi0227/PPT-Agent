package spec

const SchemaVersion = "4.0"

type Deck struct {
	SchemaVersion string          `json:"version"`
	Revision      int             `json:"revision"`
	ProjectID     string          `json:"project_id"`
	Title         string          `json:"title"`
	Goal          string          `json:"goal"`
	Audience      string          `json:"audience"`
	Language      string          `json:"language"`
	Positioning   string          `json:"positioning,omitempty"`
	Requirements  []string        `json:"requirements"`
	Prohibitions  []string        `json:"prohibitions"`
	Canvas        CanvasSettings  `json:"canvas"`
	Numbering     NumberingPolicy `json:"numbering"`
	CreatedAt     int64           `json:"created_at"`
	UpdatedAt     int64           `json:"updated_at"`
}
type CanvasSettings struct {
	AspectRatio string `json:"aspect_ratio"`
}
type NumberingPolicy struct {
	Enabled     bool     `json:"enabled"`
	HiddenRoles []string `json:"hidden_roles"`
	Format      string   `json:"format"`
}

type Outline struct {
	SchemaVersion string    `json:"version"`
	Revision      int       `json:"revision"`
	ProjectID     string    `json:"project_id"`
	Sections      []Section `json:"sections"`
	CreatedAt     int64     `json:"created_at"`
	UpdatedAt     int64     `json:"updated_at"`
}
type Section struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Purpose     string       `json:"purpose"`
	Slides      []SlideNode  `json:"slides"`
	Subsections []Subsection `json:"subsections"`
}
type Subsection struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Slides []SlideNode `json:"slides"`
}
type SlideNode struct {
	SlideID string `json:"slide_id"`
	Label   string `json:"label"`
	Role    string `json:"role"`
}

type SlideSpec struct {
	SchemaVersion string    `json:"version"`
	Revision      int       `json:"revision"`
	ProjectID     string    `json:"project_id"`
	SlideID       string    `json:"slide_id"`
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

type ProjectContentSnapshot struct {
	Deck       Deck                    `json:"deck"`
	Outline    Outline                 `json:"outline"`
	Design     Design                  `json:"design"`
	SlidesByID map[string]SlideContent `json:"slides_by_id"`
}
type SlideContent struct {
	SpecState       string                 `json:"spec_state"`
	Spec            *SlideSpec             `json:"spec"`
	HTMLState       string                 `json:"html_state"`
	HTMLRevision    int                    `json:"html_revision"`
	Materialization *MaterializationRecord `json:"materialization"`
}
type ProjectView struct {
	Deck       Deck                       `json:"deck"`
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
	Frame         MaterializationFrame    `json:"frame"`
	RenderedAt    int64                   `json:"rendered_at"`
}
type MaterializationArtifact struct {
	Revision int    `json:"revision"`
	Hash     string `json:"hash"`
}
type MaterializationSource struct {
	DeckRevision    int    `json:"deck_revision"`
	OutlineNodeHash string `json:"outline_node_hash"`
	SpecRevision    int    `json:"spec_revision"`
	DesignRevision  int    `json:"design_revision"`
	Hash            string `json:"hash"`
}
type MaterializationFrame struct {
	ContextHash string `json:"context_hash"`
}
