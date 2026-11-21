package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import tools.jackson.databind.JsonNode;

public record Command(
        String id,
        String operationId,
        String sessionId,
        String runId,
        long seq,
        CommandType type,
        CommandStatus status,
        JsonNode input,
        JsonNode result,
        JsonNode error,
        long createdAt
) {
}
