package com.dasi.ppt.agent.interfaces.controller.interaction.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.dasi.ppt.agent.interfaces.controller.history.vo.ToolDetailVo;
import com.fasterxml.jackson.annotation.JsonUnwrapped;
import tools.jackson.databind.JsonNode;

import java.util.List;

public record InteractionFormVo(
        InteractionType type,
        String version,
        @JsonUnwrapped InteractionRequestVo request
) {

    public record QuestionVo(
            String id,
            String question,
            String reason,
            List<QuestionOptionVo> options
    ) {
    }

    public record QuestionOptionVo(
            String id,
            String label,
            String description
    ) {
    }

    public record PlanFormVo(String content, List<String> steps) {
    }

    public record PermissionFormVo(
            String command,
            String stdout,
            String stderr,
            List<ToolDetailVo.ToolChangedTargetVo> changes
    ) {
    }

    public record ScopeFormVo(
            String reason,
            List<String> current,
            List<String> proposed
    ) {
    }

    public record InteractionRequestVo(
            List<QuestionVo> questions,
            PlanFormVo plan,
            PermissionFormVo permission,
            ScopeFormVo scope,
            ChangePart resource,
            JsonNode base,
            JsonNode proposal
    ) {
    }

    public record InteractionAnswerVo(
            List<QuestionAnswerVo> answers,
            String decision,
            String feedback
    ) {
    }

    public record QuestionAnswerVo(
            String questionId,
            String selectedOptionId,
            String customText,
            Boolean skipped
    ) {
    }
}
