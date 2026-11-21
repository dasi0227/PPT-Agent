package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewIssue;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewMismatch;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolResultStatus;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangeStatus;
import com.dasi.ppt.agent.interfaces.controller.interaction.vo.InteractionFormVo;
import com.fasterxml.jackson.annotation.JsonInclude;
import tools.jackson.databind.JsonNode;

import java.util.List;

public record ToolDetailVo(JsonNode arguments, ToolResultVo result) {

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record ToolResultVo(
            ToolResultStatus status,
            JsonNode content,
            String text,
            List<ToolChangedTargetVo> targets,
            ToolImageVo image,
            List<LoadedResourceVo> resources,
            ToolCommandVo command,
            ToolInteractionVo interaction,
            PreviewAssessmentVo preview
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record ToolChangedTargetVo(
            ChangePart part,
            String path,
            String slideId,
            ChangeStatus status,
            RunChangesVo.ChangeDiffVo diff
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record ToolImageVo(
            String source,
            String url,
            String slideId
    ) {
    }

    public record LoadedResourceVo(
            AssetType type,
            String id,
            String name
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record ToolCommandVo(
            String text,
            Integer exitCode,
            Long durationMs,
            String stdoutPreview,
            String stderrPreview
    ) {
    }

    public record ToolInteractionVo(
            InteractionType type,
            InteractionFormVo.InteractionRequestVo request,
            InteractionFormVo.InteractionAnswerVo answer,
            List<String> appliedScope
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record PreviewAssessmentVo(
            String slideId,
            PreviewStatus status,
            PreviewScoresVo scores,
            PreviewJudgesVo judges,
            String reason
    ) {
    }

    public record PreviewScoresVo(
            double contentCoverage,
            double expressionClarity,
            double requirementAdherence
    ) {
    }

    public record PreviewJudgesVo(PreviewMismatch mismatch, PreviewIssue issue) {
    }
}
