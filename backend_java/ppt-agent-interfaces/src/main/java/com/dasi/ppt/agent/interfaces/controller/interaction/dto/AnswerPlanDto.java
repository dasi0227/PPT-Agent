package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PlanDecision;

public record AnswerPlanDto(
        String runId,
        String toolCallId,
        String version,
        PlanDecision decision,
        String feedback
) {
}
