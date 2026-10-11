package model

import "github.com/dasi0227/PPT-Agent/backend/internal/designsystem"

type ThemeTag string

type Theme struct {
	ResourceContentState
	StyleHash   string                   `json:"style_hash"`
	Appearance  *designsystem.Appearance `json:"appearance"`
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Tags        []ThemeTag               `json:"tags"`
	Disabled    bool                     `json:"disabled"`
	CSS         string                   `json:"css,omitempty"`
	LocalPath   string                   `json:"-"`
	OpenURL     string                   `json:"open_url"`
}

type ComponentTag string

type Component struct {
	ResourceContentState
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Tags        []ComponentTag `json:"tags"`
	HTML        string         `json:"html,omitempty"`
	Disabled    bool           `json:"disabled"`
	LocalPath   string         `json:"-"`
	OpenURL     string         `json:"open_url"`
}

type SkillTag string

type RepositorySkill struct {
	ResourceContentState
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tags        []SkillTag `json:"tags"`
	Content     string     `json:"content,omitempty"`
	Disabled    bool       `json:"disabled"`
	LocalPath   string     `json:"-"`
	OpenURL     string     `json:"open_url"`
}
