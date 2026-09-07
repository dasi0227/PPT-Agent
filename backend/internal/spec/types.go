package spec

const SchemaVersion = "4.0"

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
	SchemaVersion string    `json:"version"`
	Revision      int       `json:"revision"`
	ProjectID     string    `json:"project_id"`
	SlideID       string    `json:"slide_id"`
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

type RuntimeFrameContext struct {
	SlideID    string                `json:"slide_id"`
	Canvas     RuntimeCanvas         `json:"canvas"`
	ThemeID    string                `json:"theme_id"`
	DeckTitle  string                `json:"deck_title"`
	Ordinal    int                   `json:"ordinal"`
	Total      int                   `json:"total"`
	Role       string                `json:"role"`
	Section    RuntimeFrameAncestor  `json:"section"`
	Subsection *RuntimeFrameAncestor `json:"subsection,omitempty"`
	Numbering  RuntimeFrameNumbering `json:"numbering"`
	Chrome     []ChromeItem          `json:"chrome"`
}
type RuntimeFrameAncestor struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Index int    `json:"index"`
}
type RuntimeFrameNumbering struct {
	Visible bool   `json:"visible"`
	Format  string `json:"format"`
}

type ProjectContentSnapshot struct {
	Manifest   Manifest                `json:"manifest"`
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

// Materialization is a read-only per-slide status derived from materialization.json.
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
	ManifestRevision  int    `json:"manifest_revision"`
	OutlineNodeHash   string `json:"outline_node_hash"`
	SpecRevision      int    `json:"spec_revision"`
	DesignContentHash string `json:"design_content_hash"`
	Hash              string `json:"hash"`
}
type MaterializationFrame struct {
	ContextHash string `json:"context_hash"`
}
