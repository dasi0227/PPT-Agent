package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import tools.jackson.databind.JsonNode;

public record PendingInteraction(
        String runId,
        String toolCallId,
        InteractionType type,
        String version,
        JsonNode request
) {
}
