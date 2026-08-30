package pptmutation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

var ErrRevisionConflict = errors.New("mutation revision conflict")
var ErrInvalid = errors.New("mutation invalid")

var Operations = []string{"manifest.patch", "outline.init", "outline.insert", "outline.move", "outline.update", "outline.remove", "design.write", "design.patch", "slide.spec.write", "slide.spec.patch", "slide.html.write", "slide.html.patch"}

type Workspace interface {
	Read(path string) ([]byte, error)
	Write(path string, content []byte) error
	Delete(path string) error
}
type IDGenerator func(prefix string) string

type Service struct {
	Workspace    Workspace
	ProjectID    string
	NewID        IDGenerator
	Now          func() int64
	ValidateHTML func([]byte) error
}

type Edit struct {
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}
type Position struct {
	ParentID string `json:"parent_id,omitempty"`
	BeforeID string `json:"before_id,omitempty"`
	AfterID  string `json:"after_id,omitempty"`
}
type DraftSlide struct {
	ClientRef string         `json:"client_ref"`
	Title     string         `json:"title"`
	Role      spec.SlideRole `json:"role"`
}
type DraftSubsection struct {
	ClientRef string       `json:"client_ref"`
	Title     string       `json:"title"`
	Purpose   string       `json:"purpose"`
	Slides    []DraftSlide `json:"slides"`
}
type DraftSection struct {
	ClientRef   string            `json:"client_ref"`
	Title       string            `json:"title"`
	Purpose     string            `json:"purpose"`
	Slides      []DraftSlide      `json:"slides"`
	Subsections []DraftSubsection `json:"subsections"`
}
type DraftNode struct {
	Kind        string            `json:"kind"`
	ClientRef   string            `json:"client_ref"`
	Title       string            `json:"title,omitempty"`
	Purpose     string            `json:"purpose,omitempty"`
	Role        spec.SlideRole    `json:"role,omitempty"`
	Slides      []DraftSlide      `json:"slides,omitempty"`
	Subsections []DraftSubsection `json:"subsections,omitempty"`
}
type Request struct {
	Op                 string          `json:"op"`
	ExpectedRevision   int             `json:"expected_revision,omitempty"`
	Patch              []Patch         `json:"patch,omitempty"`
	Structure          []DraftSection  `json:"structure,omitempty"`
	Node               DraftNode       `json:"node,omitempty"`
	NodeID             string          `json:"node_id,omitempty"`
	Position           Position        `json:"position,omitempty"`
	Changes            map[string]any  `json:"changes,omitempty"`
	DirectSlidesPolicy string          `json:"direct_slides_policy,omitempty"`
	ChildPolicy        string          `json:"child_policy,omitempty"`
	SlideID            string          `json:"slide_id,omitempty"`
	Spec               json.RawMessage `json:"spec,omitempty"`
	Design             json.RawMessage `json:"design,omitempty"`
	HTML               string          `json:"html,omitempty"`
	Edits              []Edit          `json:"edits,omitempty"`
}
type Result struct {
	Operation           string            `json:"operation"`
	Revisions           map[string]int    `json:"revisions"`
	Created             map[string]string `json:"created"`
	AffectedSlideIDs    []string          `json:"affected_slide_ids"`
	InvalidatedSlideIDs []string          `json:"invalidated_slide_ids"`
	InvalidatedReasons  map[string]string `json:"invalidated_reasons,omitempty"`
}

func (s Service) Apply(req Request) (Result, error) {
	if s.NewID == nil {
		s.NewID = func(prefix string) string { return model.MustShortID(prefix) }
	}
	if s.Now == nil {
		s.Now = func() int64 { return time.Now().Unix() }
	}
	result := Result{Operation: req.Op, Revisions: map[string]int{}, Created: map[string]string{}, AffectedSlideIDs: []string{}, InvalidatedSlideIDs: []string{}, InvalidatedReasons: map[string]string{}}
	switch req.Op {
	case "manifest.patch":
		return s.patchManifest(req, result)
	case "outline.init", "outline.insert", "outline.move", "outline.update", "outline.remove":
		return s.mutateOutline(req, result)
	case "design.write", "design.patch":
		return s.mutateDesign(req, result)
	case "slide.spec.write", "slide.spec.patch":
		return s.mutateSpec(req, result)
	case "slide.html.write", "slide.html.patch":
		return s.mutateHTML(req, result)
	default:
		return result, fmt.Errorf("%w: unsupported op %q", ErrInvalid, req.Op)
	}
}

func (s Service) patchManifest(req Request, out Result) (Result, error) {
	var current spec.Manifest
	if err := s.readJSON("manifest.json", &current); err != nil {
		return out, err
	}
	if err := checkRevision(req.ExpectedRevision, current.Revision); err != nil {
		return out, err
	}
	raw, _ := json.Marshal(current)
	nextRaw, err := applyPatch(raw, req.Patch, req.Op)
	if err != nil {
		return out, err
	}
	var next spec.Manifest
	if err = json.Unmarshal(nextRaw, &next); err != nil {
		return out, invalid(err)
	}
	next.SchemaVersion = spec.SchemaVersion
	next.ProjectID = s.ProjectID
	next.Revision = current.Revision + 1
	next.CreatedAt = current.CreatedAt
	next.UpdatedAt = s.Now()
	if err = spec.ValidateManifest(next); err != nil {
		return out, invalid(err)
	}
	if err = s.writeJSON("manifest.json", next); err != nil {
		return out, err
	}
	out.Revisions["manifest"] = next.Revision
	flat, _ := s.currentOutline()
	for _, loc := range spec.FlattenOutline(flat) {
		out.InvalidatedSlideIDs = append(out.InvalidatedSlideIDs, loc.Slide.SlideID)
		out.InvalidatedReasons[loc.Slide.SlideID] = "manifest_changed"
	}
	return out, nil
}

func (s Service) mutateOutline(req Request, out Result) (Result, error) {
	outline, err := s.currentOutline()
	if err != nil {
		return out, err
	}
	if err = checkRevision(req.ExpectedRevision, outline.Revision); err != nil {
		return out, err
	}
	before := spec.FlattenOutline(outline)
	beforeIDs := ids(before)
	switch req.Op {
	case "outline.init":
		if len(before) > 0 || len(outline.Sections) > 0 {
			return out, invalid(errors.New("outline.init requires an empty outline"))
		}
		refs := map[string]bool{}
		for _, d := range req.Structure {
			sec, err := s.makeSection(d, refs, out.Created)
			if err != nil {
				return out, err
			}
			outline.Sections = append(outline.Sections, sec)
		}
	case "outline.insert":
		if err = s.insertNode(&outline, req, out.Created); err != nil {
			return out, err
		}
	case "outline.move":
		if err = moveNode(&outline, req.NodeID, req.Position); err != nil {
			return out, err
		}
	case "outline.update":
		if err = updateNode(&outline, req.NodeID, req.Changes); err != nil {
			return out, err
		}
	case "outline.remove":
		removed, err := removeNode(&outline, req.NodeID, req.ChildPolicy)
		if err != nil {
			return out, err
		}
		for _, id := range removed {
			_ = s.Workspace.Delete(model.SlideSpecPath(id))
			_ = s.Workspace.Delete(model.SlideHTMLPath(id))
			_ = s.Workspace.Delete(model.SlideMaterializationPath(id))
		}
	}
	outline.SchemaVersion = spec.SchemaVersion
	outline.ProjectID = s.ProjectID
	outline.Revision++
	outline.UpdatedAt = s.Now()
	if err = spec.ValidateOutline(outline); err != nil {
		return out, invalid(err)
	}
	if err = s.writeJSON("outline.json", outline); err != nil {
		return out, err
	}
	after := spec.FlattenOutline(outline)
	out.AffectedSlideIDs = symmetric(beforeIDs, ids(after))
	if req.Op == "outline.move" || req.Op == "outline.update" {
		out.AffectedSlideIDs = ids(after)
	}
	out.Revisions["outline"] = outline.Revision
	return out, nil
}

func (s Service) makeSection(d DraftSection, refs map[string]bool, created map[string]string) (spec.Section, error) {
	if err := clientRef(d.ClientRef, refs); err != nil {
		return spec.Section{}, err
	}
	id := s.NewID("sec")
	created[d.ClientRef] = id
	sec := spec.Section{ID: id, Title: d.Title, Purpose: d.Purpose, Slides: []spec.SlideNode{}, Subsections: []spec.Subsection{}}
	if len(d.Slides) > 0 && len(d.Subsections) > 0 {
		return sec, invalid(errors.New("section cannot mix slides and subsections"))
	}
	for _, sl := range d.Slides {
		node, err := s.makeSlide(sl, refs, created)
		if err != nil {
			return sec, err
		}
		sec.Slides = append(sec.Slides, node)
	}
	for _, sub := range d.Subsections {
		if err := clientRef(sub.ClientRef, refs); err != nil {
			return sec, err
		}
		subID := s.NewID("sub")
		created[sub.ClientRef] = subID
		next := spec.Subsection{ID: subID, Title: sub.Title, Purpose: sub.Purpose, Slides: []spec.SlideNode{}}
		for _, sl := range sub.Slides {
			node, err := s.makeSlide(sl, refs, created)
			if err != nil {
				return sec, err
			}
			next.Slides = append(next.Slides, node)
		}
		sec.Subsections = append(sec.Subsections, next)
	}
	return sec, nil
}
func (s Service) makeSlide(d DraftSlide, refs map[string]bool, created map[string]string) (spec.SlideNode, error) {
	if err := clientRef(d.ClientRef, refs); err != nil {
		return spec.SlideNode{}, err
	}
	id := s.NewID("sli")
	created[d.ClientRef] = id
	return spec.SlideNode{SlideID: id, Title: d.Title, Role: d.Role}, nil
}

func (s Service) insertNode(outline *spec.Outline, req Request, created map[string]string) error {
	if req.Node.ClientRef == "" {
		return invalid(errors.New("inserted node requires client_ref"))
	}
	refs := map[string]bool{}
	switch req.Node.Kind {
	case "section":
		d := DraftSection{ClientRef: req.Node.ClientRef, Title: req.Node.Title, Purpose: req.Node.Purpose, Slides: req.Node.Slides, Subsections: req.Node.Subsections}
		node, err := s.makeSection(d, refs, created)
		if err != nil {
			return err
		}
		return insertSection(outline, node, req.Position)
	case "subsection":
		section := findSection(outline, req.Position.ParentID)
		if section == nil {
			return invalid(errors.New("subsection parent must be a section"))
		}
		if len(section.Slides) > 0 && req.DirectSlidesPolicy != "move_into_new_subsection" {
			return invalid(errors.New("direct_slides_policy is required"))
		}
		if err := clientRef(req.Node.ClientRef, refs); err != nil {
			return err
		}
		id := s.NewID("sub")
		created[req.Node.ClientRef] = id
		sub := spec.Subsection{ID: id, Title: req.Node.Title, Purpose: req.Node.Purpose, Slides: []spec.SlideNode{}}
		if len(section.Slides) > 0 {
			sub.Slides = section.Slides
			section.Slides = []spec.SlideNode{}
		}
		return insertSubsection(section, sub, req.Position)
	case "slide":
		parent, slides := findSlideParent(outline, req.Position.ParentID)
		if slides == nil {
			return invalid(errors.New("slide parent must be a section or subsection"))
		}
		_ = parent
		node, err := s.makeSlide(DraftSlide{ClientRef: req.Node.ClientRef, Title: req.Node.Title, Role: req.Node.Role}, refs, created)
		if err != nil {
			return err
		}
		return insertSlide(slides, node, req.Position)
	default:
		return invalid(errors.New("node kind must be section, subsection, or slide"))
	}
}

func (s Service) mutateDesign(req Request, out Result) (Result, error) {
	var current spec.Design
	err := s.readJSON("design.json", &current)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return out, err
	}
	if err = checkRevision(req.ExpectedRevision, current.Revision); err != nil {
		return out, err
	}
	var next spec.Design
	if req.Op == "design.write" {
		if err = strictJSON(req.Design, &next); err != nil {
			return out, invalid(err)
		}
	} else {
		raw, _ := json.Marshal(current)
		raw, err = applyPatch(raw, req.Patch, req.Op)
		if err != nil {
			return out, err
		}
		if err = json.Unmarshal(raw, &next); err != nil {
			return out, invalid(err)
		}
	}
	next.SchemaVersion = spec.SchemaVersion
	next.ProjectID = s.ProjectID
	next.Revision = max(current.Revision+1, 1)
	next.CreatedAt = current.CreatedAt
	if next.CreatedAt == 0 {
		next.CreatedAt = s.Now()
	}
	next.UpdatedAt = s.Now()
	if err = spec.ValidateDesign(next); err != nil {
		return out, invalid(err)
	}
	if err = s.writeJSON("design.json", next); err != nil {
		return out, err
	}
	out.Revisions["design"] = next.Revision
	outline, _ := s.currentOutline()
	out.InvalidatedSlideIDs = ids(spec.FlattenOutline(outline))
	for _, id := range out.InvalidatedSlideIDs {
		out.InvalidatedReasons[id] = "design_changed"
	}
	return out, nil
}

func (s Service) mutateSpec(req Request, out Result) (Result, error) {
	outline, err := s.currentOutline()
	if err != nil {
		return out, err
	}
	if _, ok := spec.FindSlide(outline, req.SlideID); !ok {
		return out, invalid(errors.New("slide_id is not in outline"))
	}
	path := model.SlideSpecPath(req.SlideID)
	var current spec.SlideSpec
	readErr := s.readJSON(path, &current)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return out, readErr
	}
	if err = checkRevision(req.ExpectedRevision, current.Revision); err != nil {
		return out, err
	}
	var next spec.SlideSpec
	if req.Op == "slide.spec.write" {
		if err = strictJSON(req.Spec, &next); err != nil {
			return out, invalid(err)
		}
	} else {
		if readErr != nil {
			return out, invalid(errors.New("cannot patch pending spec"))
		}
		raw, _ := json.Marshal(current)
		raw, err = applyPatch(raw, req.Patch, req.Op)
		if err != nil {
			return out, err
		}
		if err = json.Unmarshal(raw, &next); err != nil {
			return out, invalid(err)
		}
	}
	next.SchemaVersion = spec.SchemaVersion
	next.ProjectID = s.ProjectID
	next.SlideID = req.SlideID
	next.Revision = max(current.Revision+1, 1)
	next.CreatedAt = current.CreatedAt
	if next.CreatedAt == 0 {
		next.CreatedAt = s.Now()
	}
	next.UpdatedAt = s.Now()
	if err = spec.ValidateSlideSpec(next); err != nil {
		return out, invalid(err)
	}
	if err = s.writeJSON(path, next); err != nil {
		return out, err
	}
	out.Revisions["spec"] = next.Revision
	out.AffectedSlideIDs = []string{req.SlideID}
	out.InvalidatedSlideIDs = []string{req.SlideID}
	out.InvalidatedReasons[req.SlideID] = "spec_changed"
	return out, nil
}

func (s Service) mutateHTML(req Request, out Result) (Result, error) {
	path := model.SlideHTMLPath(req.SlideID)
	outline, err := s.currentOutline()
	if err != nil {
		return out, err
	}
	if _, ok := spec.FindSlide(outline, req.SlideID); !ok {
		return out, invalid(errors.New("slide_id is not in outline"))
	}
	candidate := req.HTML
	if req.Op == "slide.html.patch" {
		raw, err := s.Workspace.Read(path)
		if err != nil {
			return out, err
		}
		candidate = string(raw)
		for _, edit := range req.Edits {
			count := strings.Count(candidate, edit.OldText)
			if count != 1 {
				return out, invalid(fmt.Errorf("old_text must match exactly once, got %d", count))
			}
			candidate = strings.Replace(candidate, edit.OldText, edit.NewText, 1)
		}
	}
	if strings.Contains(candidate, "data-runtime-page-number") || strings.Contains(candidate, "data-page-number") {
		return out, invalid(errors.New("slide HTML must not contain a static page number"))
	}
	if s.ValidateHTML != nil {
		if err = s.ValidateHTML([]byte(candidate)); err != nil {
			return out, invalid(err)
		}
	}
	if err = s.Workspace.Write(path, []byte(candidate)); err != nil {
		return out, err
	}
	out.AffectedSlideIDs = []string{req.SlideID}
	out.InvalidatedSlideIDs = []string{req.SlideID}
	out.InvalidatedReasons[req.SlideID] = "html_changed"
	return out, nil
}

func (s Service) currentOutline() (spec.Outline, error) {
	var o spec.Outline
	err := s.readJSON("outline.json", &o)
	return o, err
}
func (s Service) readJSON(path string, out any) error {
	raw, err := s.Workspace.Read(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
func (s Service) writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return s.Workspace.Write(path, raw)
}
func strictJSON(raw []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	return nil
}
func checkRevision(expected, current int) error {
	if expected > 0 && expected != current {
		return ErrRevisionConflict
	}
	return nil
}
func invalid(err error) error { return fmt.Errorf("%w: %v", ErrInvalid, err) }
func clientRef(ref string, seen map[string]bool) error {
	if strings.TrimSpace(ref) == "" || seen[ref] {
		return invalid(errors.New("client_ref must be non-empty and unique"))
	}
	seen[ref] = true
	return nil
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func ids(flat []spec.SlideLocation) []string {
	out := make([]string, 0, len(flat))
	for _, loc := range flat {
		out = append(out, loc.Slide.SlideID)
	}
	return out
}
func symmetric(a, b []string) []string {
	seen := map[string]int{}
	for _, id := range a {
		seen[id]++
	}
	for _, id := range b {
		seen[id]++
	}
	out := []string{}
	for id, n := range seen {
		if n == 1 {
			out = append(out, id)
		}
	}
	return out
}

func findSection(outline *spec.Outline, id string) *spec.Section {
	for i := range outline.Sections {
		if outline.Sections[i].ID == id {
			return &outline.Sections[i]
		}
	}
	return nil
}
func findSlideParent(outline *spec.Outline, id string) (string, *[]spec.SlideNode) {
	for i := range outline.Sections {
		s := &outline.Sections[i]
		if s.ID == id && len(s.Subsections) == 0 {
			return s.ID, &s.Slides
		}
		for j := range s.Subsections {
			if s.Subsections[j].ID == id {
				return s.Subsections[j].ID, &s.Subsections[j].Slides
			}
		}
	}
	return "", nil
}
func validateAnchor(before, after string) error {
	if before != "" && after != "" {
		return invalid(errors.New("before_id and after_id are mutually exclusive"))
	}
	return nil
}
func insertionIndex(length int, before, after string, index func(string) int) (int, error) {
	if err := validateAnchor(before, after); err != nil {
		return 0, err
	}
	if before != "" {
		i := index(before)
		if i < 0 {
			return 0, invalid(errors.New("before_id is not a sibling"))
		}
		return i, nil
	}
	if after != "" {
		i := index(after)
		if i < 0 {
			return 0, invalid(errors.New("after_id is not a sibling"))
		}
		return i + 1, nil
	}
	return length, nil
}
func insertSection(o *spec.Outline, n spec.Section, p Position) error {
	i, err := insertionIndex(len(o.Sections), p.BeforeID, p.AfterID, func(id string) int {
		for i := range o.Sections {
			if o.Sections[i].ID == id {
				return i
			}
		}
		return -1
	})
	if err != nil {
		return err
	}
	o.Sections = append(o.Sections, spec.Section{})
	copy(o.Sections[i+1:], o.Sections[i:])
	o.Sections[i] = n
	return nil
}
func insertSubsection(s *spec.Section, n spec.Subsection, p Position) error {
	i, err := insertionIndex(len(s.Subsections), p.BeforeID, p.AfterID, func(id string) int {
		for i := range s.Subsections {
			if s.Subsections[i].ID == id {
				return i
			}
		}
		return -1
	})
	if err != nil {
		return err
	}
	s.Subsections = append(s.Subsections, spec.Subsection{})
	copy(s.Subsections[i+1:], s.Subsections[i:])
	s.Subsections[i] = n
	return nil
}
func insertSlide(slides *[]spec.SlideNode, n spec.SlideNode, p Position) error {
	i, err := insertionIndex(len(*slides), p.BeforeID, p.AfterID, func(id string) int {
		for i := range *slides {
			if (*slides)[i].SlideID == id {
				return i
			}
		}
		return -1
	})
	if err != nil {
		return err
	}
	*slides = append(*slides, spec.SlideNode{})
	copy((*slides)[i+1:], (*slides)[i:])
	(*slides)[i] = n
	return nil
}

func moveNode(o *spec.Outline, id string, p Position) error {
	for i := range o.Sections {
		if o.Sections[i].ID == id {
			node := o.Sections[i]
			o.Sections = append(o.Sections[:i], o.Sections[i+1:]...)
			return insertSection(o, node, p)
		}
	}
	for si := range o.Sections {
		for subi := range o.Sections[si].Subsections {
			if o.Sections[si].Subsections[subi].ID == id {
				node := o.Sections[si].Subsections[subi]
				o.Sections[si].Subsections = append(o.Sections[si].Subsections[:subi], o.Sections[si].Subsections[subi+1:]...)
				target := findSection(o, p.ParentID)
				if target == nil || len(target.Slides) > 0 {
					return invalid(errors.New("subsection target must be a grouped or empty section"))
				}
				return insertSubsection(target, node, p)
			}
		}
	}
	var node spec.SlideNode
	found := false
	for si := range o.Sections {
		slides := &o.Sections[si].Slides
		for i := range *slides {
			if (*slides)[i].SlideID == id {
				node = (*slides)[i]
				*slides = append((*slides)[:i], (*slides)[i+1:]...)
				found = true
				break
			}
		}
		for subi := range o.Sections[si].Subsections {
			slides = &o.Sections[si].Subsections[subi].Slides
			for i := range *slides {
				if (*slides)[i].SlideID == id {
					node = (*slides)[i]
					*slides = append((*slides)[:i], (*slides)[i+1:]...)
					found = true
					break
				}
			}
		}
	}
	if !found {
		return invalid(errors.New("node_id not found"))
	}
	_, target := findSlideParent(o, p.ParentID)
	if target == nil {
		return invalid(errors.New("slide target parent is invalid"))
	}
	return insertSlide(target, node, p)
}
func updateNode(o *spec.Outline, id string, changes map[string]any) error {
	for si := range o.Sections {
		s := &o.Sections[si]
		if s.ID == id {
			for k, v := range changes {
				switch k {
				case "title":
					s.Title = fmt.Sprint(v)
				case "purpose":
					s.Purpose = fmt.Sprint(v)
				default:
					return invalid(errors.New("invalid section change"))
				}
			}
			return nil
		}
		for subi := range s.Subsections {
			sub := &s.Subsections[subi]
			if sub.ID == id {
				for k, v := range changes {
					switch k {
					case "title":
						sub.Title = fmt.Sprint(v)
					case "purpose":
						sub.Purpose = fmt.Sprint(v)
					default:
						return invalid(errors.New("invalid subsection change"))
					}
				}
				return nil
			}
		}
		for i := range s.Slides {
			if s.Slides[i].SlideID == id {
				return updateSlide(&s.Slides[i], changes)
			}
		}
		for subi := range s.Subsections {
			for i := range s.Subsections[subi].Slides {
				if s.Subsections[subi].Slides[i].SlideID == id {
					return updateSlide(&s.Subsections[subi].Slides[i], changes)
				}
			}
		}
	}
	return invalid(errors.New("node_id not found"))
}
func updateSlide(slide *spec.SlideNode, changes map[string]any) error {
	for k, v := range changes {
		switch k {
		case "title":
			slide.Title = fmt.Sprint(v)
		case "role":
			slide.Role = spec.SlideRole(fmt.Sprint(v))
		default:
			return invalid(errors.New("invalid slide change"))
		}
	}
	return nil
}
func removeNode(o *spec.Outline, id, policy string) ([]string, error) {
	for si := range o.Sections {
		s := &o.Sections[si]
		if s.ID == id {
			if len(s.Slides) > 0 || len(s.Subsections) > 0 {
				return nil, invalid(errors.New("section must be empty"))
			}
			o.Sections = append(o.Sections[:si], o.Sections[si+1:]...)
			return nil, nil
		}
		for i := range s.Slides {
			if s.Slides[i].SlideID == id {
				s.Slides = append(s.Slides[:i], s.Slides[i+1:]...)
				return []string{id}, nil
			}
		}
		for subi := range s.Subsections {
			sub := &s.Subsections[subi]
			if sub.ID == id {
				if len(sub.Slides) > 0 {
					if policy != "promote_to_section" || len(s.Subsections) != 1 {
						return nil, invalid(errors.New("non-empty subsection requires child_policy promote_to_section as the last subsection"))
					}
					s.Slides = sub.Slides
					s.Subsections = []spec.Subsection{}
					return nil, nil
				}
				s.Subsections = append(s.Subsections[:subi], s.Subsections[subi+1:]...)
				return nil, nil
			}
			for i := range sub.Slides {
				if sub.Slides[i].SlideID == id {
					sub.Slides = append(sub.Slides[:i], sub.Slides[i+1:]...)
					return []string{id}, nil
				}
			}
		}
	}
	return nil, invalid(errors.New("node_id not found"))
}
