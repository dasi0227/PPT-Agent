package workflow

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

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
	if err := ctx.Err(); err != nil {
		return internalToolFailure(err)
	}
	if id, ok := input.Args["slide_id"].(string); ok {
		entry, err := renderimage.Latest(input.ProjectDir, input.Context.Project.ID, id)
		if err != nil {
			return renderedImageFailure(err, id)
		}
		dependencyHash, err := currentImageDependencyHash(input.Context, input.ProjectDir, input.Session, entry)
		if err != nil {
			return renderedImageFailure(err, id)
		}
		if entry.DependencyHash == "" || entry.DependencyHash != dependencyHash {
			return renderedImageFailure(fmt.Errorf("%w: referenced assets, appearance or visible decorations changed, or the screenshot has no dependency evidence", errScreenshotInvalid), id)
		}
		if _, _, err := renderimage.Read(ctx, input.ProjectDir, input.Context.Project.ID, entry.ImageRef()); err != nil {
			return renderedImageFailure(err, id)
		}
		result := SuccessfulToolResult("rendered slide image read")
		result.Data = map[string]any{"image_source": "render", "slide_id": id, "image_path": entry.ImagePath(), "image_url": fmt.Sprintf("/api/v1/runs/%s/screenshots/%s", entry.RunID, entry.ScreenshotID)}
		result.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: entry.ImageRef(), MIMEType: "image/png", Detail: "high"}}
		return result
	}
	id := stringValue(input.Args["attachment_id"])
	meta, _, err := attachment.Read(ctx, input.ProjectDir, input.Context.Project.ID, id, "original")
	if err != nil {
		return attachmentReadFailure(err, id)
	}
	result := SuccessfulToolResult("image attachment read")
	result.Data = map[string]any{"image_source": "attachment", "attachment_id": id, "image_name": meta.OriginalName, "image_url": fmt.Sprintf("/api/v1/projects/%s/attachments/%s/content?variant=original", input.Context.Project.ID, id)}
	result.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: meta.ImageRef(input.Context.Project.ID, "original"), MIMEType: meta.MediaType, Detail: "high"}}
	return result
}

func attachmentReadFailure(err error, id string) ToolResult {
	var value *attachment.Error
	if errors.As(err, &value) {
		reason := "Attachment " + id + " is empty, damaged or inconsistent with its file type."
		measure := "Replace this attachment with a valid PNG or JPEG original; do not repeat an unchanged image read."
		switch value.Code {
		case attachment.NotFound:
			return detailedToolFailure("IMAGE_NOT_FOUND", "Attachment "+id+" original does not exist in the current project.", map[string]any{"measure": "Use an existing attachment_id from the current project, or upload the missing image before reading it."})
		case attachment.TooLarge:
			reason = "Attachment " + id + " exceeds the 10 MiB image limit."
			measure = "Compress the image to at most 10 MiB and upload it again."
		case attachment.DimensionsExceeded:
			reason = "Attachment " + id + " has invalid dimensions or exceeds 40 megapixels."
			measure = "Resize the image to positive dimensions and at most 40 megapixels, then upload it again."
		case attachment.TypeUnsupported:
			reason = "Attachment " + id + " is not a supported PNG or JPEG image."
		case attachment.Invalid:
		default:
			return internalToolFailure(err)
		}
		return detailedToolFailure("IMAGE_INVALID", reason, map[string]any{"measure": measure})
	}
	if errors.Is(err, fs.ErrNotExist) {
		return detailedToolFailure("IMAGE_NOT_FOUND", "Attachment "+id+" original no longer exists.", map[string]any{"measure": "Use an existing attachment_id or upload the missing original before reading it."})
	}
	return internalToolFailure(err)
}

func renderedImageFailure(err error, id string) ToolResult {
	if errors.Is(err, errScreenshotInvalid) {
		return detailedToolFailure("IMAGE_INVALID", "Screenshot for "+id+" is not current: "+err.Error(), map[string]any{"measure": "Restore the reported page location or visible pixel dependencies, then call render_slide for this page. Do not repeat an unchanged screenshot read."})
	}
	if errors.Is(err, errScreenshotPageMissing) {
		return detailedToolFailure("IMAGE_NOT_FOUND", "Page "+id+" does not exist in the current outline.", map[string]any{"measure": "Use an existing page ID from the current outline. Create missing pages with edit_outline when disclosed before rendering their screenshots."})
	}
	if errors.Is(err, fs.ErrNotExist) {
		return detailedToolFailure("IMAGE_NOT_FOUND", "Screenshot for "+id+" does not exist.", map[string]any{"measure": "Use a current page ID from the outline, then call render_slide before reading its screenshot. Create missing pages first when needed; do not repeat reads of missing images."})
	}
	if errors.Is(err, renderimage.ErrUnavailable) {
		return detailedToolFailure("IMAGE_INVALID", "Screenshot for "+id+" cannot be used: "+err.Error(), map[string]any{"measure": "Render this page again to replace the invalid screenshot with a valid PNG within the image size and dimension limits."})
	}
	return internalToolFailure(err)
}
