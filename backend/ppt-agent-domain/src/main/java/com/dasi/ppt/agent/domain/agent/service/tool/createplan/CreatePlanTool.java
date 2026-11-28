package com.dasi.ppt.agent.domain.agent.service.tool.createplan;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("create_plan")
public class CreatePlanTool implements Tool<CreatePlanRequest> {

    @Override
    public ToolResponse execute(CreatePlanRequest request) {
        throw new UnsupportedOperationException();
    }
}
