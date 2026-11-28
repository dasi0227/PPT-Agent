package com.dasi.ppt.agent.domain.agent.service.tool.updateplan;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("update_plan")
public class UpdatePlanTool implements Tool<UpdatePlanRequest> {

    @Override
    public ToolResponse execute(UpdatePlanRequest request) {
        throw new UnsupportedOperationException();
    }
}
