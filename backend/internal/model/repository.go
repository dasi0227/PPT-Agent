package model

type Theme struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CSS         string `json:"css,omitempty"`
	CSSURL      string `json:"css_url"`
	LocalPath   string `json:"-"`
	OpenURL     string `json:"open_url"`
}

type ComponentTag string

const (
	ComponentTagCard       ComponentTag = "card"
	ComponentTagMetric     ComponentTag = "metric"
	ComponentTagComparison ComponentTag = "comparison"
	ComponentTagQuote      ComponentTag = "quote"
	ComponentTagList       ComponentTag = "list"
	ComponentTagChart      ComponentTag = "chart"
	ComponentTagProcess    ComponentTag = "process"
	ComponentTagTimeline   ComponentTag = "timeline"
	ComponentTagOther      ComponentTag = "other"
)

func (tag ComponentTag) Valid() bool {
	switch tag {
	case ComponentTagCard, ComponentTagMetric, ComponentTagComparison, ComponentTagQuote,
		ComponentTagList, ComponentTagChart, ComponentTagProcess, ComponentTagTimeline, ComponentTagOther:
		return true
	default:
		return false
	}
}

type Component struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Tags        []ComponentTag `json:"tags"`
	Kind        string         `json:"kind,omitempty"`
	HTML        string         `json:"html,omitempty"`
	LocalPath   string         `json:"-"`
	OpenURL     string         `json:"open_url"`
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
