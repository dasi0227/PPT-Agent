package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Artifact string

const (
	ArtifactSpec Artifact = "spec"
	ArtifactPPT  Artifact = "ppt"
)

type ScopeLevel string

const (
	ScopeSlide ScopeLevel = "slide"
	ScopeDeck  ScopeLevel = "deck"
)

type RunMode string

const (
	ModeTalk    RunMode = "talk"
	ModeAsk     RunMode = "ask"
	ModePlan    RunMode = "plan"
	ModeExecute RunMode = "execute"
)

type RunLanguage string

const (
	LanguageChinese RunLanguage = "zh-CN"
	LanguageEnglish RunLanguage = "en-US"
)

type SlideRange string

const (
	SlideRangeFiveToEight         SlideRange = "5-8"
	SlideRangeNineToFifteen       SlideRange = "9-15"
	SlideRangeSixteenToTwentyFive SlideRange = "16-25"
	SlideRangeTwentySixPlus       SlideRange = "26+"
)

type RunScope struct {
	Artifact Artifact   `json:"artifact"`
	Level    ScopeLevel `json:"level"`
	SlideID  string     `json:"slide_id,omitempty"`
}

type RunOptions struct {
	Language RunLanguage `json:"language,omitempty"`
	Range    SlideRange  `json:"range,omitempty"`
}

const (
	MaxRunSkills      = 3
	MaxRunComponents  = 8
	MaxMentionedPages = 8
)

var slideIDPattern = regexp.MustCompile(`^sli_[A-Za-z0-9_-]+$`)

type RunSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	LocalPath   string `json:"local_path,omitempty"`
	OpenURL     string `json:"open_url,omitempty"`
}

type PublicSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled,omitempty"`
	LocalPath   string `json:"local_path,omitempty"`
	OpenURL     string `json:"open_url,omitempty"`
}

type RunComponent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	HTML        string `json:"html"`
	LocalPath   string `json:"-"`
	OpenURL     string `json:"open_url,omitempty"`
}

// MentionedPage is a lightweight pointer to a slide explicitly named by the user.
type MentionedPage struct {
	Kind      string `json:"kind"`
	SlideID   string `json:"slide_id"`
	Ordinal   int    `json:"ordinal"`
	Title     string `json:"title,omitempty"`
	SpecState string `json:"spec_state"`
	HTMLState string `json:"html_state"`
}

type RunCommand struct {
	Scope                    RunScope        `json:"scope"`
	Mode                     RunMode         `json:"mode"`
	Instruction              string          `json:"instruction"`
	Options                  RunOptions      `json:"options,omitempty"`
	Skills                   []RunSkill      `json:"skills,omitempty"`
	Components               []RunComponent  `json:"components,omitempty"`
	MentionedPages           []MentionedPage `json:"mentioned_pages,omitempty"`
	DroppedMentionedSlideIDs []string        `json:"dropped_mentioned_slide_ids,omitempty"`
}

var ErrInvalidRunCommand = errors.New("invalid run command")

func (c RunCommand) Validate() error {
	if c.Scope.Artifact != ArtifactSpec && c.Scope.Artifact != ArtifactPPT {
		return fmt.Errorf("%w: unsupported artifact %q", ErrInvalidRunCommand, c.Scope.Artifact)
	}
	if c.Scope.Level != ScopeSlide && c.Scope.Level != ScopeDeck {
		return fmt.Errorf("%w: unsupported level %q", ErrInvalidRunCommand, c.Scope.Level)
	}
	if c.Scope.Level == ScopeSlide && strings.TrimSpace(c.Scope.SlideID) == "" {
		return fmt.Errorf("%w: slide_id is required for slide scope", ErrInvalidRunCommand)
	}
	if c.Scope.Level == ScopeDeck && c.Scope.SlideID != "" {
		return fmt.Errorf("%w: slide_id is forbidden for deck scope", ErrInvalidRunCommand)
	}
	if c.Scope.SlideID == "current" {
		return fmt.Errorf("%w: current must be resolved to a stable slide_id", ErrInvalidRunCommand)
	}
	switch c.Mode {
	case ModeTalk, ModeAsk, ModePlan, ModeExecute:
	default:
		return fmt.Errorf("%w: unsupported mode %q", ErrInvalidRunCommand, c.Mode)
	}
	if strings.TrimSpace(c.Instruction) == "" {
		return fmt.Errorf("%w: instruction is required", ErrInvalidRunCommand)
	}
	switch c.Options.Language {
	case "", LanguageChinese, LanguageEnglish:
	default:
		return fmt.Errorf("%w: unsupported language %q", ErrInvalidRunCommand, c.Options.Language)
	}
	switch c.Options.Range {
	case "", SlideRangeFiveToEight, SlideRangeNineToFifteen, SlideRangeSixteenToTwentyFive, SlideRangeTwentySixPlus:
	default:
		return fmt.Errorf("%w: unsupported range %q", ErrInvalidRunCommand, c.Options.Range)
	}
	if c.Options.Range != "" && c.Scope.Level != ScopeDeck {
		return fmt.Errorf("%w: range is only valid for deck scope", ErrInvalidRunCommand)
	}
	if len(c.Skills) > MaxRunSkills {
		return fmt.Errorf("%w: at most %d skills may be selected", ErrInvalidRunCommand, MaxRunSkills)
	}
	seenSkills := map[string]bool{}
	for _, skill := range c.Skills {
		if strings.TrimSpace(skill.ID) == "" || strings.TrimSpace(skill.Name) == "" ||
			strings.TrimSpace(skill.Description) == "" || strings.TrimSpace(skill.Content) == "" {
			return fmt.Errorf("%w: selected skills must be complete", ErrInvalidRunCommand)
		}
		if seenSkills[skill.ID] {
			return fmt.Errorf("%w: selected skills must be unique", ErrInvalidRunCommand)
		}
		seenSkills[skill.ID] = true
	}
	if len(c.Components) > MaxRunComponents {
		return fmt.Errorf("%w: at most %d components may be referenced", ErrInvalidRunCommand, MaxRunComponents)
	}
	seenComponents := map[string]bool{}
	for _, component := range c.Components {
		name := strings.TrimSpace(component.Name)
		if name == "" || strings.TrimSpace(component.HTML) == "" {
			return fmt.Errorf("%w: referenced components must include name and html", ErrInvalidRunCommand)
		}
		if seenComponents[name] {
			return fmt.Errorf("%w: referenced component names must be unique", ErrInvalidRunCommand)
		}
		seenComponents[name] = true
	}
	if len(c.MentionedPages) > MaxMentionedPages {
		return fmt.Errorf("%w: at most %d pages may be mentioned", ErrInvalidRunCommand, MaxMentionedPages)
	}
	if len(c.MentionedPages) > 0 && c.Scope.Level != ScopeDeck {
		return fmt.Errorf("%w: mentioned pages are only valid for deck scope", ErrInvalidRunCommand)
	}
	seenPages := map[string]bool{}
	for _, page := range c.MentionedPages {
		if page.Kind != "slide" {
			return fmt.Errorf("%w: mentioned page kind must be slide", ErrInvalidRunCommand)
		}
		if !slideIDPattern.MatchString(page.SlideID) {
			return fmt.Errorf("%w: mentioned page has invalid slide_id", ErrInvalidRunCommand)
		}
		if seenPages[page.SlideID] {
			return fmt.Errorf("%w: mentioned pages must be unique", ErrInvalidRunCommand)
		}
		seenPages[page.SlideID] = true
	}
	return nil
}

func (c RunCommand) PublicSkills() []PublicSkill {
	if len(c.Skills) == 0 {
		return nil
	}
	skills := make([]PublicSkill, 0, len(c.Skills))
	for _, skill := range c.Skills {
		skills = append(skills, PublicSkill{
			ID: skill.ID, Name: skill.Name, Description: skill.Description,
			LocalPath: skill.LocalPath, OpenURL: skill.OpenURL,
		})
	}
	return skills
}

func (c RunCommand) PublicComponents() []PublicLoadedResource {
	if len(c.Components) == 0 {
		return nil
	}
	components := make([]PublicLoadedResource, 0, len(c.Components))
	for _, component := range c.Components {
		components = append(components, PublicLoadedResource{
			Kind: "component", ID: component.ID, Name: component.Name, OpenURL: component.OpenURL,
		})
	}
	return components
}

func (r SlideRange) Contains(count int) bool {
	switch r {
	case SlideRangeFiveToEight:
		return count >= 5 && count <= 8
	case SlideRangeNineToFifteen:
		return count >= 9 && count <= 15
	case SlideRangeSixteenToTwentyFive:
		return count >= 16 && count <= 25
	case SlideRangeTwentySixPlus:
		return count >= 26
	default:
		return false
	}
}

type MaterializationState string

const (
	MaterializationNotMaterialized MaterializationState = "not_materialized"
	MaterializationFresh           MaterializationState = "fresh"
	MaterializationSpecStale       MaterializationState = "spec_stale"
	MaterializationDesignStale     MaterializationState = "design_stale"
	MaterializationUnknown         MaterializationState = "unknown"
)

type MaterializationRevisions struct {
	SlideHTML int `json:"slide_html"`
	Outline   int `json:"source_outline"`
	SlideSpec int `json:"source_spec"`
	Design    int `json:"source_design"`
}
