package model

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type ScopeObject string

const (
	ScopeObjectSpec         ScopeObject = "spec"
	ScopeObjectHTML         ScopeObject = "html"
	ScopeObjectPresentation ScopeObject = "presentation"
	ScopeObjectGlobal       ScopeObject = "global"
)

type ScopeSelectionKind string

const (
	ScopeCurrentPage    ScopeSelectionKind = "current_page"
	ScopeAllPages       ScopeSelectionKind = "all_pages"
	ScopeCustomPages    ScopeSelectionKind = "custom_pages"
	ScopeCustomSections ScopeSelectionKind = "custom_sections"
)

type ScopeSelectionInput struct {
	Kind           ScopeSelectionKind `json:"kind"`
	CurrentSlideID string             `json:"current_slide_id,omitempty"`
	SlideIDs       []string           `json:"slide_ids,omitempty"`
	SectionIDs     []string           `json:"section_ids,omitempty"`
}

type CreateRunScopeInput struct {
	Object    ScopeObject         `json:"object"`
	Selection ScopeSelectionInput `json:"selection"`
}

type ScopeSource struct {
	Kind       ScopeSelectionKind `json:"kind"`
	SectionIDs []string           `json:"section_ids,omitempty"`
}

type RunMode string

const (
	ModeChat    RunMode = "chat"
	ModeGrill   RunMode = "grill"
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
	Object                  ScopeObject `json:"object"`
	SlideIDs                []string    `json:"slide_ids"`
	Source                  ScopeSource `json:"source"`
	IncludeRunCreatedSlides bool        `json:"include_run_created_slides"`
	Revision                int64       `json:"revision"`
}

func NewRunScope(object ScopeObject, kind ScopeSelectionKind, slideIDs ...string) RunScope {
	return RunScope{
		Object: object, SlideIDs: append([]string{}, slideIDs...), Source: ScopeSource{Kind: kind},
		IncludeRunCreatedSlides: kind == ScopeAllPages, Revision: 1,
	}
}

func (s RunScope) Validate() error {
	switch s.Object {
	case ScopeObjectSpec, ScopeObjectHTML, ScopeObjectPresentation, ScopeObjectGlobal:
	default:
		return fmt.Errorf("unsupported scope object %q", s.Object)
	}
	switch s.Source.Kind {
	case ScopeCurrentPage, ScopeAllPages, ScopeCustomPages, ScopeCustomSections:
	default:
		return fmt.Errorf("unsupported scope source %q", s.Source.Kind)
	}
	if s.Revision < 1 {
		return errors.New("scope revision must be at least 1")
	}
	if len(s.SlideIDs) == 0 && s.Object != ScopeObjectGlobal && s.Source.Kind != ScopeAllPages {
		return errors.New("scope requires at least one slide_id")
	}
	seen := make(map[string]bool, len(s.SlideIDs))
	for _, id := range s.SlideIDs {
		if !slideIDPattern.MatchString(id) || id == "current" {
			return fmt.Errorf("invalid stable slide_id %q", id)
		}
		if seen[id] {
			return fmt.Errorf("duplicate slide_id %q", id)
		}
		seen[id] = true
	}
	if s.Source.Kind == ScopeCustomSections && len(s.Source.SectionIDs) == 0 {
		return errors.New("custom_sections scope requires section_ids")
	}
	if s.Source.Kind != ScopeCustomSections && len(s.Source.SectionIDs) > 0 {
		return errors.New("section_ids are only valid for custom_sections scope")
	}
	if s.Object == ScopeObjectGlobal && s.Source.Kind != ScopeAllPages {
		return errors.New("global scope must use all_pages")
	}
	if s.IncludeRunCreatedSlides != (s.Source.Kind == ScopeAllPages) {
		return errors.New("include_run_created_slides must match all_pages scope")
	}
	return nil
}

func (s RunScope) ContainsSlide(slideID string) bool {
	if s.Object == ScopeObjectGlobal {
		return true
	}
	for _, candidate := range s.SlideIDs {
		if candidate == slideID {
			return true
		}
	}
	return false
}

func (s RunScope) AllowsSpec() bool {
	return s.Object == ScopeObjectSpec || s.Object == ScopeObjectPresentation || s.Object == ScopeObjectGlobal
}

func (s RunScope) AllowsHTML() bool {
	return s.Object == ScopeObjectHTML || s.Object == ScopeObjectPresentation || s.Object == ScopeObjectGlobal
}

func (s RunScope) AllowsGlobal() bool { return s.Object == ScopeObjectGlobal }

func (s RunScope) IsSinglePage() bool { return !s.AllowsGlobal() && len(s.SlideIDs) == 1 }

func (s RunScope) Equal(other RunScope) bool {
	return s.Object == other.Object && s.Source.Kind == other.Source.Kind &&
		slices.Equal(s.Source.SectionIDs, other.Source.SectionIDs) &&
		slices.Equal(s.SlideIDs, other.SlideIDs) &&
		s.IncludeRunCreatedSlides == other.IncludeRunCreatedSlides && s.Revision == other.Revision
}

type RunOptions struct {
	Language RunLanguage `json:"language,omitempty"`
	Range    SlideRange  `json:"range,omitempty"`
}

const (
	MaxRunSkills      = 3
	MaxRunComponents  = 8
	MaxMentionedPages = 8
	MaxRunAttachments = 8
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

// AttachmentReference is the immutable, message-level snapshot of a project image.
// The project filesystem owns the bytes; Runs and steering messages retain this
// metadata so history remains interpretable after a context compaction.
type AttachmentReference struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	MediaType    string `json:"media_type"`
	Extension    string `json:"extension"`
	SizeBytes    int64  `json:"size_bytes"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type RunCommand struct {
	Scope                    RunScope              `json:"scope"`
	Mode                     RunMode               `json:"mode"`
	Instruction              string                `json:"instruction"`
	Options                  RunOptions            `json:"options,omitempty"`
	Skills                   []RunSkill            `json:"skills,omitempty"`
	Components               []RunComponent        `json:"components,omitempty"`
	MentionedPages           []MentionedPage       `json:"mentioned_pages,omitempty"`
	DroppedMentionedSlideIDs []string              `json:"dropped_mentioned_slide_ids,omitempty"`
	Attachments              []AttachmentReference `json:"attachments,omitempty"`
}

var ErrInvalidRunCommand = errors.New("invalid run command")

func (c RunCommand) Validate() error {
	if err := c.Scope.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRunCommand, err)
	}
	switch c.Mode {
	case ModeChat, ModeGrill, ModePlan, ModeExecute:
	default:
		return fmt.Errorf("%w: unsupported mode %q", ErrInvalidRunCommand, c.Mode)
	}
	if strings.TrimSpace(c.Instruction) == "" && len(c.Attachments) == 0 {
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
	if c.Options.Range != "" && c.Scope.Source.Kind != ScopeAllPages {
		return fmt.Errorf("%w: range is only valid for all_pages scope", ErrInvalidRunCommand)
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
	if len(c.Attachments) > MaxRunAttachments {
		return fmt.Errorf("%w: at most %d attachments may be referenced", ErrInvalidRunCommand, MaxRunAttachments)
	}
	seenAttachments := map[string]bool{}
	for _, attachment := range c.Attachments {
		if !strings.HasPrefix(attachment.ID, "att_") || strings.TrimSpace(attachment.OriginalName) == "" ||
			(attachment.MediaType != "image/png" && attachment.MediaType != "image/jpeg" && attachment.MediaType != "image/webp") ||
			attachment.SizeBytes <= 0 || attachment.Width <= 0 || attachment.Height <= 0 {
			return fmt.Errorf("%w: attachment snapshot is invalid", ErrInvalidRunCommand)
		}
		if seenAttachments[attachment.ID] {
			return fmt.Errorf("%w: attachments must be unique", ErrInvalidRunCommand)
		}
		seenAttachments[attachment.ID] = true
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
