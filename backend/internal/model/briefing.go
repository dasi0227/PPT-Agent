package model

type BriefingKind string

const (
	BriefingKickoff BriefingKind = "kickoff"
	BriefingHandoff BriefingKind = "handoff"
)

func (k BriefingKind) Valid() bool {
	return k == BriefingKickoff || k == BriefingHandoff
}

type BriefingVersion struct {
	BriefingID string       `json:"briefing_id"`
	ThreadID   string       `json:"thread_id"`
	ProjectID  string       `json:"project_id"`
	Kind       BriefingKind `json:"kind"`
	VersionNo  int          `json:"version_no"`
	Content    string       `json:"content"`
	Feedback   string       `json:"feedback"`
	CreatedAt  int64        `json:"created_at"`
}

type Briefing struct {
	BriefingID string            `json:"briefing_id"`
	ThreadID   string            `json:"thread_id"`
	ProjectID  string            `json:"project_id"`
	Kind       BriefingKind      `json:"kind"`
	Versions   []BriefingVersion `json:"versions"`
	UpdatedAt  int64             `json:"updated_at"`
}
