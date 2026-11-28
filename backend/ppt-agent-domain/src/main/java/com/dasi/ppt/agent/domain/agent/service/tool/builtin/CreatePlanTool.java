package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.result.CreatePlanResult;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("create_plan")
@RequiredArgsConstructor
public class CreatePlanTool implements Tool {

    private final InteractionService interactionService;

    @Override
    public CreatePlanResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
