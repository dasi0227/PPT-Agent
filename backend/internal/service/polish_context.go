package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// Polish needs enough context to locate "this page", not the material needed to
// execute the draft. Read only the outline, including the active session's view.
func buildPolishInput(project model.Project, params PolishParams, instruction string) (string, error) {
	var raw []byte
	var err error
	if session := workflow.ActiveRunSession(project.WorkDir); session != nil {
		raw, err = session.ReadPath(".outline.json")
	} else {
		raw, err = os.ReadFile(filepath.Join(project.WorkDir, ".outline.json"))
	}
	var outline spec.Outline
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err == nil {
		if err := json.Unmarshal(raw, &outline); err != nil {
			return "", err
		}
	}
	scope, err := resolveRunScope(spec.ProjectContentSnapshot{Outline: outline}, params.ScopeInput)
	if err != nil {
		return "", err
	}
	if err := (model.RunCommand{Scope: scope, Mode: params.Mode, Instruction: instruction}).Validate(); err != nil {
		return "", err
	}
	type page struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	target := struct {
		Selection    model.ScopeSelectionKind `json:"selection"`
		PageCount    int                      `json:"page_count"`
		Pages        []page                   `json:"pages,omitempty"`
		PagesOmitted int                      `json:"pages_omitted,omitempty"`
	}{Selection: scope.Source.Kind, PageCount: len(scope.SlideIDs)}
	// All-pages drafts do not need a deck inventory. For a selection, send only
	// a few human-readable page labels and an explicit omitted count.
	if scope.Source.Kind != model.ScopeAllPages {
		selected := make(map[string]bool, len(scope.SlideIDs))
		for _, id := range scope.SlideIDs {
			selected[id] = true
		}
		for index, loc := range spec.FlattenOutline(outline) {
			if selected[loc.Slide.ID] && len(target.Pages) < 6 {
				target.Pages = append(target.Pages, page{Number: index + 1, Title: commandExcerpt(loc.Slide.Title, 80)})
			}
		}
		target.PagesOmitted = len(scope.SlideIDs) - len(target.Pages)
	}
	input := struct {
		Draft            string        `json:"draft"`
		RevisionFeedback string        `json:"revision_feedback,omitempty"`
		ProjectTitle     string        `json:"project_title,omitempty"`
		Mode             model.RunMode `json:"mode"`
		Target           any           `json:"target"`
	}{instruction, params.Feedback, commandExcerpt(project.Title, 120), params.Mode, target}
	raw, err = json.Marshal(input)
	return string(raw), err
}
