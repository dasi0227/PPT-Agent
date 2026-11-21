package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ResourceDecision;
import tools.jackson.databind.JsonNode;

public record AnswerResourceDto(
        String runId,
        String toolCallId,
        String version,
        ResourceDecision decision,
        JsonNode draft,
        String feedback
) {
}
