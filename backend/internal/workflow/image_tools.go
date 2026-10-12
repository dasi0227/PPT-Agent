package workflow

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

type readImageTool struct{}

func (readImageTool) Schema() ToolSchema {
	return tooldefinition.Get("read_image")
}

func (t readImageTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	if id, ok := input.Args["slide_id"].(string); ok {
		images, err := latestRenderedImages(input.Context, input.ProjectDir, input.Session)
		if err != nil {
			return failedToolResult(CodeResourceInvalid, "Unable to read the current render index.")
		}
		for _, image := range images {
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
		return detailedToolFailure(CodeResourceNotFound, "截图不存在或已过期，请先调用 render_slide。", map[string]any{"measure": "Call render_slide for this slide_id before reading its screenshot."})
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
