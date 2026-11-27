package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Command {
    private String id;
    private String operationId;
    private String sessionId;
    private String runId;
    private long seq;
    private CommandType type;
    private CommandStatus status;
    private JsonNode input;
    private JsonNode result;
    private JsonNode error;
    private long createdAt;
}
