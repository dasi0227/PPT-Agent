package model

type Theme struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	CSS         string   `json:"css,omitempty"`
	CSSURL      string   `json:"css_url"`
	LocalPath   string   `json:"-"`
	OpenURL     string   `json:"open_url"`
}

type Component struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Kind        string   `json:"kind,omitempty"`
	HTML        string   `json:"html,omitempty"`
	LocalPath   string   `json:"-"`
	OpenURL     string   `json:"open_url"`
}

type RepositorySkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content,omitempty"`
	Disabled    bool   `json:"disabled"`
	LocalPath   string `json:"-"`
	OpenURL     string `json:"open_url"`
}
