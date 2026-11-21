package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PermissionDecision;

public record AnswerPermissionDto(
        String runId,
        String toolCallId,
        String version,
        PermissionDecision decision,
        String feedback
) {
}
