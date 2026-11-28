package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PermissionDecision;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.dasi.ppt.agent.domain.project.model.entity.DeckChange;
import lombok.Getter;
import lombok.experimental.SuperBuilder;
import tools.jackson.databind.JsonNode;

import java.util.List;

@Getter
@SuperBuilder
public final class RunCommandResult extends ToolResult {

    private final Integer exitCode;
    private final String stdout;
    private final String stderr;
    private final List<DeckChange> changes;
    private final JsonNode request;
    private final PermissionDecision decision;
    private final String feedback;
}
