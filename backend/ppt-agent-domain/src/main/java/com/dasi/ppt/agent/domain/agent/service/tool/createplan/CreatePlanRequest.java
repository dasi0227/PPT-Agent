package com.dasi.ppt.agent.domain.agent.service.tool.createplan;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class CreatePlanRequest extends ToolRequest {

    private final String content;
    private final List<String> steps;
}
