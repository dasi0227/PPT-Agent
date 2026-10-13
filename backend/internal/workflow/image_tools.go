package workflow

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
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
		outlineRaw, _, err := readArtifact(input.ProjectDir, input.Session, outlineRef(input.Context))
		if err != nil {
			return renderedImageFailure(err)
		}
		parsed, err := spec.ParseStrictSourceJSON(outlineRaw, "outline")
		if err != nil {
			return savedResourceInvalid(err)
		}
		if _, exists := spec.FindSlide(*parsed.(*spec.Outline), id); !exists {
			return renderedImageFailure(renderimage.ErrUnavailable)
		}
		entry, err := renderimage.Latest(input.ProjectDir, input.Context.Project.ID, id)
		if err != nil {
			return renderedImageFailure(err)
		}
		proof, err := currentRenderProof(input.Context, input.ProjectDir, input.Session, id, entry.SourceHash)
		if err != nil {
			return renderedImageFailure(err)
		}
		if entry.DependencyHash != proof.DependencyHash() {
			return renderedImageFailure(renderimage.ErrUnavailable)
		}
		if _, _, err := renderimage.Read(ctx, input.ProjectDir, input.Context.Project.ID, entry.ImageRef()); err != nil {
			return renderedImageFailure(err)
		}
		result := SuccessfulToolResult("rendered slide image read")
		result.Data = map[string]any{"image_source": "render", "slide_id": id, "image_path": entry.ImagePath(), "image_url": fmt.Sprintf("/api/v1/runs/%s/screenshots/%s", entry.RunID, entry.ScreenshotID)}
		result.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: entry.ImageRef(), MIMEType: "image/png", Detail: "high"}}
		return result
	}
	id := stringValue(input.Args["attachment_id"])
	meta, _, err := attachment.Read(ctx, input.ProjectDir, input.Context.Project.ID, id, "original")
	if err != nil {
		return attachmentReadFailure(err)
	}
	result := SuccessfulToolResult("image attachment read")
	result.Data = map[string]any{"image_source": "attachment", "attachment_id": id, "image_name": meta.OriginalName, "image_url": fmt.Sprintf("/api/v1/projects/%s/attachments/%s/content?variant=original", input.Context.Project.ID, id)}
	result.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: meta.ImageRef(input.Context.Project.ID, "original"), MIMEType: meta.MediaType, Detail: "high"}}
	return result
}

func attachmentReadFailure(err error) ToolResult {
	var value *attachment.Error
	if errors.As(err, &value) {
		return failedToolResult(string(value.Code), value.Error())
	}
	return internalToolFailure(err)
}

func renderedImageFailure(err error) ToolResult {
	var failure *model.AgentError
	if errors.As(err, &failure) && failure.Code == "RESOURCE_CONTENT_INVALID" {
		return savedResourceInvalid(err)
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, renderimage.ErrUnavailable) ||
		(errors.As(err, &failure) && (failure.Code == CodeContentConflict || failure.Code == CodeResourceNotFound || failure.Code == "OUTLINE_NOT_INITIALIZED")) {
		return detailedToolFailure(CodeResourceNotFound, "截图不存在或已过期，请先调用 render_slide。", map[string]any{"measure": "Use a page's current id from the outline, then call render_slide before reading its screenshot."})
	}
	return internalToolFailure(err)
}
