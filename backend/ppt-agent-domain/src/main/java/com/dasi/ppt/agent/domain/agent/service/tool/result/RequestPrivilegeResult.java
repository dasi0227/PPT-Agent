package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ScopeDecision;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;
import tools.jackson.databind.JsonNode;

import java.util.List;

@Getter
@SuperBuilder
public final class RequestPrivilegeResult extends ToolResult {

    private final JsonNode request;
    private final ScopeDecision decision;
    private final String feedback;
    private final List<String> appliedScope;
}
