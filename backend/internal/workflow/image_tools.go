package workflow

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

type readImageTool struct{}

func (readImageTool) Schema() ToolSchema {
	parameters := objectSchema(nil, map[string]any{
		"attachment_id": map[string]any{"type": "string", "pattern": "^att_[A-Za-z0-9_-]{1,128}$", "description": "ID of an uploaded image from attachment context, used to view its original image. Supply this or slide_id, never both; do not pass a path or URL."},
		"slide_id":      map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$", "description": "Stable page ID whose latest valid rendered screenshot should be viewed. Supply this or attachment_id, never both. If no current screenshot exists, call render_slide first."},
	})
	parameters["oneOf"] = []any{
		map[string]any{"required": []string{"attachment_id"}, "not": map[string]any{"required": []string{"slide_id"}}},
		map[string]any{"required": []string{"slide_id"}, "not": map[string]any{"required": []string{"attachment_id"}}},
	}
	return ToolSchema{Name: "read_image", OutputSchema: toolOutputSchema("read_image"), Description: "Read an uploaded original image by attachment_id, or the latest valid screenshot by slide_id. Supply exactly one. Missing or stale screenshots require render_slide first. Read images are retained through context compaction and recovery within the current run. Use attachment context addresses for HTML embedding.", Parameters: parameters}
}

func (t readImageTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	if id, ok := input.Args["slide_id"].(string); ok {
		for _, image := range latestRenderedImages(input.Context, input.ProjectDir, input.Session) {
			if image.SlideID != id || image.Stale {
				continue
			}
			entry, err := renderimage.Latest(input.ProjectDir, input.Context.Project.ID, id)
			if err != nil || entry.ImagePath() != image.ImagePath {
				break
			}
			if _, _, err := renderimage.Read(ctx, input.ProjectDir, input.Context.Project.ID, entry.ImageRef()); err != nil {
				break
			}
			result := SuccessfulToolResult("rendered slide image read")
			result.Data = map[string]any{"image_source": "render", "slide_id": id, "image_path": entry.ImagePath(), "image_url": fmt.Sprintf("/api/v1/runs/%s/screenshots/%s", entry.RunID, entry.ScreenshotID)}
			result.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: entry.ImageRef(), MIMEType: "image/png", Detail: "high"}}
			return result
		}
		return detailedToolFailure(CodeResourceNotFound, "截图不存在或已过期，请先调用 render_slide。", map[string]any{"next_action": "Call render_slide for this slide_id before reading its screenshot."})
	}
	id := stringValue(input.Args["attachment_id"])
	meta, _, err := attachment.Read(ctx, input.ProjectDir, input.Context.Project.ID, id, "original")
	if err != nil {
		return failedToolResult(attachmentErrorCode(err), "attachment is unavailable")
	}
	result := SuccessfulToolResult("image attachment read")
	result.Data = map[string]any{"image_source": "attachment", "attachment_id": id, "image_name": meta.OriginalName, "image_url": fmt.Sprintf("/api/v1/projects/%s/attachments/%s/content?variant=original", input.Context.Project.ID, id)}
	result.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: meta.ImageRef(input.Context.Project.ID, "original"), MIMEType: meta.MediaType, Detail: "high"}}
	return result
}

func attachmentErrorCode(err error) string {
	if value, ok := err.(*attachment.Error); ok {
		return string(value.Code)
	}
	return CodeResourceNotFound
}
