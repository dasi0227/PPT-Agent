package com.dasi.ppt.agent.domain.agent.service.tool.updateplan;

import com.dasi.ppt.agent.domain.agent.model.entity.PlanStep;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class UpdatePlanRequest extends ToolRequest {

    private final List<PlanStep> steps;
}
