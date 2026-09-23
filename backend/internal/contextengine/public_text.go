package contextengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func PublicTextContext(pack ContextPack) model.PublicTextContext {
	display := publicTextContext(pack.Project.ID, pack.Outline.Outline, pack.Command.Instruction)
	display.HiddenValues = append(display.HiddenValues, pack.Manifest.RunID, pack.Manifest.ThreadID, pack.Manifest.ContextID)
	return display
}

func ProjectPublicTextContext(project model.Project, source string) model.PublicTextContext {
	var outline pptspec.Outline
	if raw, err := os.ReadFile(filepath.Join(project.WorkDir, "outline.json")); err == nil {
		_ = json.Unmarshal(raw, &outline)
	}
	display := publicTextContext(project.ID, outline, source)
	display.HiddenValues = append(display.HiddenValues, project.WorkDir)
	return display
}

func publicTextContext(projectID string, outline pptspec.Outline, source string) model.PublicTextContext {
	c := model.PublicTextContext{Pages: map[string]string{}, HiddenValues: []string{projectID}, SourceText: source}
	for index, loc := range pptspec.FlattenOutline(outline) {
		c.Pages[loc.Slide.SlideID] = fmt.Sprintf("第 %d 页", index+1)
	}
	return c
}

func PublicSourceText(messages []llm.Message) string {
	parts := []string{}
	for _, message := range messages {
		if message.Role == llm.RoleUser && (message.Metadata == nil || message.Metadata.Origin == "user") {
			for _, part := range message.Content {
				if part.Type != "text" || attachmentDescriptionKind(part.Text) != "" {
					continue
				}
				text := part.Text
				// Only the comment is user-authored. Snapshot IDs and page data
				// must not exempt internal vocabulary from display sanitization.
				for _, tag := range []string{"selected_dom", "selected_dom_reference"} {
					open, close := "<"+tag+">", "</"+tag+">"
					if strings.HasPrefix(text, open) && strings.HasSuffix(text, close) {
						var selection struct {
							Comment string `json:"comment"`
						}
						_ = json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(text, open), close)), &selection)
						text = selection.Comment
						break
					}
				}
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}
