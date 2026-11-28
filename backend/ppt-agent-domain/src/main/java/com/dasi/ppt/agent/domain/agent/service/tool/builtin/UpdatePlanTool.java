package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.result.UpdatePlanResult;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("update_plan")
@RequiredArgsConstructor
public class UpdatePlanTool implements Tool {

    private final IRunRepository runRepository;

    @Override
    public UpdatePlanResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
