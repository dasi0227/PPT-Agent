package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.model.entity.Plan;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PlanDecision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;
import tools.jackson.databind.JsonNode;

@Getter
@SuperBuilder
public final class CreatePlanResult extends ToolResult {

    private final Plan plan;
    private final RunMode mode;
    private final JsonNode request;
    private final PlanDecision decision;
    private final String feedback;
}
