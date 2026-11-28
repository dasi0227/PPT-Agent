package com.dasi.ppt.agent.domain.agent.service.tool.createplan;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("create_plan")
public class CreatePlanTool implements Tool<CreatePlanRequest> {

    @Override
    public ToolResult execute(CreatePlanRequest request) {
        throw new UnsupportedOperationException();
    }
}
