package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.AgentTool;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("update_plan")
@RequiredArgsConstructor
public class UpdatePlanTool implements AgentTool {

    private final IRunRepository runRepository;

    @Override
    public ToolName getName() {
        return ToolName.UPDATE_PLAN;
    }

    @Override
    public ToolResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
