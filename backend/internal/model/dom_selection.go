package model

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	MaxDOMSelections       = 8
	MaxDOMTargets          = 50
	MaxDOMSelectionBytes   = 128 * 1024
	MaxDOMSelectionsBytes  = 256 * 1024
	MaxDOMSelectionComment = 500
)

var (
	ErrDOMSelectionInvalid        = errors.New("DOM_SELECTION_INVALID")
	ErrDOMSelectionLimit          = errors.New("DOM_SELECTION_LIMIT")
	ErrDOMSelectionTooLarge       = errors.New("DOM_SELECTION_TOO_LARGE")
	ErrDOMSelectionCommentTooLong = errors.New("DOM_SELECTION_COMMENT_TOO_LONG")
	ErrReferenceOrderInvalid      = errors.New("REFERENCE_ORDER_INVALID")
	selectionIDPattern            = regexp.MustCompile(`^sel_[A-Za-z0-9_-]{1,128}$`)
)

type DOMSelectionKind string
type DOMSelectionStatus string

const (
	DOMSelectionElement        DOMSelectionKind   = "element"
	DOMSelectionRegion         DOMSelectionKind   = "region"
	DOMSelectionActive         DOMSelectionStatus = "active"
	DOMSelectionContentDeleted DOMSelectionStatus = "content_deleted"
	DOMSelectionPageDeleted    DOMSelectionStatus = "page_deleted"
)

type CanvasSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type CanvasRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type DOMFingerprint struct {
	Tag             string        `json:"tag"`
	StableID        string        `json:"stable_id,omitempty"`
	Classes         []string      `json:"classes,omitempty"`
	KeyAttributes   []string      `json:"key_attributes,omitempty"`
	SiblingIndex    int           `json:"sibling_index"`
	TextSummaryHash string        `json:"text_summary_hash,omitempty"`
	Ancestors       []DOMAncestor `json:"ancestors,omitempty"`
}

type DOMAncestor struct {
	Tag          string `json:"tag"`
	StableID     string `json:"stable_id,omitempty"`
	ClassSummary string `json:"class_summary,omitempty"`
	SiblingIndex int    `json:"sibling_index"`
}

type DOMEdges struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

type DOMBoxModel struct {
	Content CanvasRect `json:"content"`
	Padding DOMEdges   `json:"padding"`
	Border  DOMEdges   `json:"border"`
	Margin  DOMEdges   `json:"margin"`
}

type DOMTarget struct {
	TargetID           string             `json:"target_id"`
	Fingerprint        DOMFingerprint     `json:"fingerprint"`
	CandidateSelectors []string           `json:"candidate_selectors,omitempty"`
	Tag                string             `json:"tag"`
	Attributes         map[string]string  `json:"attributes,omitempty"`
	TextSummary        string             `json:"text_summary,omitempty"`
	OuterHTML          string             `json:"outer_html,omitempty"`
	OuterHTMLTruncated bool               `json:"outer_html_truncated,omitempty"`
	Ancestors          []DOMAncestor      `json:"ancestors,omitempty"`
	ParentTargetID     string             `json:"parent_target_id,omitempty"`
	Rect               CanvasRect         `json:"rect"`
	BoxModel           DOMBoxModel        `json:"box_model"`
	ComputedStyle      map[string]string  `json:"computed_style,omitempty"`
	Status             DOMSelectionStatus `json:"status,omitempty"`
}

type ChromeTarget struct {
	Type      string     `json:"type"`
	Placement string     `json:"placement"`
	Style     string     `json:"style"`
	Text      string     `json:"text"`
	Rect      CanvasRect `json:"rect"`
}

type DOMSelection struct {
	SelectionID   string             `json:"selection_id"`
	MarkerNo      int                `json:"marker_no"`
	Kind          DOMSelectionKind   `json:"kind"`
	Comment       string             `json:"comment"`
	SlideID       string             `json:"slide_id"`
	HTMLRevision  int                `json:"html_revision"`
	HTMLHash      string             `json:"html_hash"`
	Canvas        CanvasSize         `json:"canvas"`
	Rect          CanvasRect         `json:"rect"`
	Status        DOMSelectionStatus `json:"status"`
	DOMTargets    []DOMTarget        `json:"dom_targets,omitempty"`
	ChromeTargets []ChromeTarget     `json:"chrome_targets,omitempty"`
}

type ReferenceOrderItem struct {
	Kind  string `json:"kind"`
	RefID string `json:"ref_id"`
}

type PublicDOMSelection struct {
	SelectionID string             `json:"selection_id"`
	MarkerNo    int                `json:"marker_no"`
	Comment     string             `json:"comment"`
	Status      DOMSelectionStatus `json:"status"`
}

func PublicDOMSelections(values []DOMSelection) []PublicDOMSelection {
	if len(values) == 0 {
		return nil
	}
	out := make([]PublicDOMSelection, 0, len(values))
	for _, value := range values {
		out = append(out, PublicDOMSelection{SelectionID: value.SelectionID, MarkerNo: value.MarkerNo, Comment: value.Comment, Status: value.Status})
	}
	return out
}

func (r CanvasRect) valid() bool {
	values := []float64{r.X, r.Y, r.Width, r.Height}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return r.X >= 0 && r.Y >= 0 && r.Width >= 0 && r.Height >= 0 && r.X+r.Width <= 1920.001 && r.Y+r.Height <= 1080.001
}

func (s DOMSelection) Validate() error {
	if !selectionIDPattern.MatchString(s.SelectionID) || s.MarkerNo <= 0 || !slideIDPattern.MatchString(s.SlideID) ||
		(s.Kind != DOMSelectionElement && s.Kind != DOMSelectionRegion) ||
		(s.Status != DOMSelectionActive && s.Status != DOMSelectionContentDeleted && s.Status != DOMSelectionPageDeleted) ||
		s.HTMLRevision < 0 || strings.TrimSpace(s.HTMLHash) == "" || s.Canvas.Width != 1920 || s.Canvas.Height != 1080 || !s.Rect.valid() {
		return ErrDOMSelectionInvalid
	}
	if s.Rect.Width <= 0 || s.Rect.Height <= 0 {
		return ErrDOMSelectionInvalid
	}
	if utf8.RuneCountInString(s.Comment) > MaxDOMSelectionComment {
		return ErrDOMSelectionCommentTooLong
	}
	if len(s.DOMTargets) > MaxDOMTargets {
		return ErrDOMSelectionLimit
	}
	if len(s.DOMTargets)+len(s.ChromeTargets) == 0 && s.Status == DOMSelectionActive {
		return ErrDOMSelectionInvalid
	}
	seen := map[string]bool{}
	for _, target := range s.DOMTargets {
		if err := target.validate(); err != nil {
			return err
		}
		if seen[target.TargetID] {
			return ErrDOMSelectionInvalid
		}
		seen[target.TargetID] = true
	}
	for _, target := range s.DOMTargets {
		if target.ParentTargetID != "" && !seen[target.ParentTargetID] {
			return ErrDOMSelectionInvalid
		}
	}
	allowedChrome := map[string]bool{"page_number": true, "section_marker": true, "key_message": true, "deck_title": true}
	for _, target := range s.ChromeTargets {
		if !allowedChrome[target.Type] || !target.Rect.valid() || target.Rect.Width <= 0 || target.Rect.Height <= 0 || len(target.Text) > 1000 || len(target.Placement) > 64 || len(target.Style) > 256 {
			return ErrDOMSelectionInvalid
		}
	}
	raw, _ := json.Marshal(struct {
		DOM    []DOMTarget    `json:"dom_targets"`
		Chrome []ChromeTarget `json:"chrome_targets"`
	}{s.DOMTargets, s.ChromeTargets})
	if len(raw) > MaxDOMSelectionBytes {
		return ErrDOMSelectionTooLarge
	}
	return nil
}

var allowedComputedStyles = map[string]bool{
	"display": true, "position": true, "top": true, "right": true, "bottom": true, "left": true, "width": true, "height": true,
	"min-width": true, "min-height": true, "max-width": true, "max-height": true, "box-sizing": true, "overflow": true, "overflow-x": true, "overflow-y": true,
	"flex": true, "flex-direction": true, "flex-wrap": true, "align-items": true, "align-content": true, "justify-content": true, "justify-items": true, "gap": true, "row-gap": true, "column-gap": true,
	"grid-template-columns": true, "grid-template-rows": true, "grid-column": true, "grid-row": true,
	"font-family": true, "font-size": true, "font-weight": true, "font-style": true, "line-height": true, "letter-spacing": true, "text-align": true, "text-transform": true, "color": true,
	"background": true, "background-color": true, "border": true, "border-radius": true, "opacity": true, "transform": true, "transform-origin": true, "clip-path": true, "z-index": true,
}

func (t DOMTarget) validate() error {
	if strings.TrimSpace(t.TargetID) == "" || len(t.TargetID) > 128 || strings.TrimSpace(t.Tag) == "" || len(t.Tag) > 64 || !t.Rect.valid() || t.Rect.Width <= 0 || t.Rect.Height <= 0 || !t.BoxModel.Content.valid() || len(t.TextSummary) > 1000 || len(t.OuterHTML) > 16*1024 || len(t.CandidateSelectors) > 3 || len(t.Ancestors) > 8 {
		return ErrDOMSelectionInvalid
	}
	if err := t.Fingerprint.validate(t.Tag); err != nil {
		return err
	}
	for _, selector := range t.CandidateSelectors {
		if strings.TrimSpace(selector) == "" || len(selector) > 512 {
			return ErrDOMSelectionInvalid
		}
	}
	for _, ancestor := range t.Ancestors {
		if err := ancestor.validate(); err != nil {
			return err
		}
	}
	if !t.BoxModel.Padding.valid(false) || !t.BoxModel.Border.valid(false) || !t.BoxModel.Margin.valid(true) {
		return ErrDOMSelectionInvalid
	}
	if t.Status != "" && t.Status != DOMSelectionActive && t.Status != DOMSelectionContentDeleted {
		return ErrDOMSelectionInvalid
	}
	if strings.Contains(strings.ToLower(t.OuterHTML), "<script") || strings.Contains(strings.ToLower(t.OuterHTML), " srcdoc=") || regexp.MustCompile(`(?i)\son[a-z]+\s*=`).MatchString(t.OuterHTML) {
		return ErrDOMSelectionInvalid
	}
	for key, value := range t.Attributes {
		lower := strings.ToLower(key)
		allowed := lower == "id" || lower == "class" || lower == "role" || lower == "title" || lower == "alt" || lower == "href" || lower == "src" || strings.HasPrefix(lower, "aria-") || strings.HasPrefix(lower, "data-")
		if !allowed || len(key) > 128 || len(value) > 512 || strings.HasPrefix(lower, "on") || lower == "srcdoc" || lower == "value" || lower == "checked" || lower == "selected" || dangerousURL(value) {
			return ErrDOMSelectionInvalid
		}
	}
	for key, value := range t.ComputedStyle {
		if !allowedComputedStyles[key] || len(value) > 512 {
			return ErrDOMSelectionInvalid
		}
	}
	return nil
}

func (f DOMFingerprint) validate(targetTag string) error {
	if strings.TrimSpace(f.Tag) == "" || !strings.EqualFold(f.Tag, targetTag) || len(f.Tag) > 64 || f.SiblingIndex < 0 || len(f.StableID) > 128 || len(f.TextSummaryHash) > 128 || len(f.Classes) > 16 || len(f.KeyAttributes) > 16 || len(f.Ancestors) > 8 {
		return ErrDOMSelectionInvalid
	}
	for _, value := range append(append([]string{}, f.Classes...), f.KeyAttributes...) {
		if strings.TrimSpace(value) == "" || len(value) > 512 {
			return ErrDOMSelectionInvalid
		}
	}
	for _, ancestor := range f.Ancestors {
		if err := ancestor.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (a DOMAncestor) validate() error {
	if strings.TrimSpace(a.Tag) == "" || len(a.Tag) > 64 || len(a.StableID) > 128 || len(a.ClassSummary) > 256 || a.SiblingIndex < 0 {
		return ErrDOMSelectionInvalid
	}
	return nil
}

func (e DOMEdges) valid(allowNegative bool) bool {
	for _, value := range []float64{e.Top, e.Right, e.Bottom, e.Left} {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1920 || (!allowNegative && value < 0) {
			return false
		}
	}
	return true
}

func dangerousURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "blob:")
}

func ValidateDOMSelections(selections []DOMSelection, attachments []AttachmentReference, order []ReferenceOrderItem) error {
	if len(selections) > MaxDOMSelections {
		return ErrDOMSelectionLimit
	}
	seenSelections, seenMarkers, total := map[string]bool{}, map[int]bool{}, 0
	for _, selection := range selections {
		if err := selection.Validate(); err != nil {
			return err
		}
		if seenSelections[selection.SelectionID] || seenMarkers[selection.MarkerNo] {
			return ErrDOMSelectionInvalid
		}
		seenSelections[selection.SelectionID], seenMarkers[selection.MarkerNo] = true, true
		raw, _ := json.Marshal(struct {
			DOM    []DOMTarget    `json:"dom_targets"`
			Chrome []ChromeTarget `json:"chrome_targets"`
		}{selection.DOMTargets, selection.ChromeTargets})
		total += len(raw)
	}
	if total > MaxDOMSelectionsBytes {
		return ErrDOMSelectionTooLarge
	}
	if len(order) != len(selections)+len(attachments) {
		return ErrReferenceOrderInvalid
	}
	wanted, actual := map[string]bool{}, map[string]bool{}
	for _, selection := range selections {
		wanted["dom:"+selection.SelectionID] = true
	}
	for _, attachment := range attachments {
		wanted["image:"+attachment.ID] = true
	}
	for _, item := range order {
		key := item.Kind + ":" + item.RefID
		if (item.Kind != "dom" && item.Kind != "image") || !wanted[key] || actual[key] {
			return ErrReferenceOrderInvalid
		}
		actual[key] = true
	}
	return nil
}

func NormalizeReferenceOrder(selections []DOMSelection, attachments []AttachmentReference, order []ReferenceOrderItem) []ReferenceOrderItem {
	if len(order) > 0 {
		return order
	}
	// Internal callers without user references legitimately have no order. User-facing
	// API paths always provide an explicit list whenever references are present.
	out := make([]ReferenceOrderItem, 0, len(selections)+len(attachments))
	for _, attachment := range attachments {
		out = append(out, ReferenceOrderItem{Kind: "image", RefID: attachment.ID})
	}
	for _, selection := range selections {
		out = append(out, ReferenceOrderItem{Kind: "dom", RefID: selection.SelectionID})
	}
	return out
}

func SortedSelectionSlideIDs(values []DOMSelection) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value.Status != DOMSelectionPageDeleted {
			seen[value.SlideID] = true
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
