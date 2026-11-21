package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ScopeDecision;

public record AnswerScopeDto(
        String runId,
        String toolCallId,
        String version,
        ScopeDecision decision,
        String feedback
) {
}
