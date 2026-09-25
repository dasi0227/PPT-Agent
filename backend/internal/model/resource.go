package model

// Resource is the database-owned identity and metadata of a file-backed asset.
type Resource struct {
	Type           string   `json:"type"`
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	NormalizedName string   `json:"-"`
	Description    string   `json:"description"`
	Disabled       bool     `json:"disabled"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
	Tags           []string `json:"tags"`
}

type ResourceContentState struct {
	ContentState string `json:"content_state"`
	ContentError string `json:"content_error,omitempty"`
}

type Snippet struct {
	Resource
	ResourceContentState
	Content string `json:"content"`
	OpenURL string `json:"open_url"`
}
