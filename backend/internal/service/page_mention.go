package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

var mentionedSlideIDPattern = regexp.MustCompile(`^sli_[A-Za-z0-9_-]+$`)

func resolveMentionedPages(
	snapshot spec.ProjectContentSnapshot,
	ids []string,
) ([]model.MentionedPage, []string, error) {
	if len(ids) > model.MaxMentionedPages {
		return nil, nil, pageSelectionError(fmt.Errorf("at most %d pages may be mentioned", model.MaxMentionedPages))
	}

	locations := make(map[string]spec.SlideLocation)
	for _, location := range spec.FlattenOutline(snapshot.Outline) {
		locations[location.Slide.SlideID] = location
	}

	seen := make(map[string]bool, len(ids))
	pages := make([]model.MentionedPage, 0, len(ids))
	dropped := make([]string, 0)
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if !mentionedSlideIDPattern.MatchString(id) {
			return nil, nil, pageSelectionError(fmt.Errorf("invalid slide_id %q", rawID))
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		location, exists := locations[id]
		if !exists {
			dropped = append(dropped, id)
			continue
		}
		content := snapshot.SlidesByID[id]
		pages = append(pages, model.MentionedPage{
			Kind: "slide", SlideID: id, Ordinal: location.Ordinal,
			Title: location.Slide.Title, SpecState: content.SpecState, HTMLState: content.HTMLState,
		})
	}
	return pages, dropped, nil
}

func pageSelectionError(err error) *model.AgentError {
	agentErr := model.NewAgentError("PAGE_SELECTION_INVALID", "create_run", err)
	agentErr.Details["max_pages"] = model.MaxMentionedPages
	return agentErr
}
