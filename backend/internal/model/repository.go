package model

type ThemeTag string

const (
	ThemeTagMinimal    ThemeTag = "minimal"
	ThemeTagBusiness   ThemeTag = "business"
	ThemeTagTechnology ThemeTag = "technology"
	ThemeTagCool       ThemeTag = "cool"
	ThemeTagWarm       ThemeTag = "warm"
	ThemeTagOther      ThemeTag = "other"
)

func (tag ThemeTag) Valid() bool {
	switch tag {
	case ThemeTagMinimal, ThemeTagBusiness, ThemeTagTechnology, ThemeTagCool, ThemeTagWarm, ThemeTagOther:
		return true
	default:
		return false
	}
}

type Theme struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tags        []ThemeTag `json:"tags"`
	CSS         string     `json:"css,omitempty"`
	CSSURL      string     `json:"css_url"`
	LocalPath   string     `json:"-"`
	OpenURL     string     `json:"open_url"`
}

type ComponentTag string

const (
	ComponentTagCard    ComponentTag = "card"
	ComponentTagChart   ComponentTag = "chart"
	ComponentTagTable   ComponentTag = "table"
	ComponentTagList    ComponentTag = "list"
	ComponentTagProcess ComponentTag = "process"
	ComponentTagMetric  ComponentTag = "metric"
	ComponentTagOther   ComponentTag = "other"
)

func (tag ComponentTag) Valid() bool {
	switch tag {
	case ComponentTagCard, ComponentTagChart, ComponentTagTable, ComponentTagList,
		ComponentTagProcess, ComponentTagMetric, ComponentTagOther:
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
	HTML        string         `json:"html,omitempty"`
	Disabled    bool           `json:"disabled"`
	LocalPath   string         `json:"-"`
	OpenURL     string         `json:"open_url"`
}

type SkillTag string

const (
	SkillTagWorkflow    SkillTag = "workflow"
	SkillTagMethodology SkillTag = "methodology"
	SkillTagManual      SkillTag = "manual"
	SkillTagExperience  SkillTag = "experience"
	SkillTagOther       SkillTag = "other"
)

func (tag SkillTag) Valid() bool {
	switch tag {
	case SkillTagWorkflow, SkillTagMethodology, SkillTagManual, SkillTagExperience, SkillTagOther:
		return true
	default:
		return false
	}
}

type RepositorySkill struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tags        []SkillTag `json:"tags"`
	Content     string     `json:"content,omitempty"`
	Disabled    bool       `json:"disabled"`
	LocalPath   string     `json:"-"`
	OpenURL     string     `json:"open_url"`
}
