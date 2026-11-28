package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.result.AskUserResult;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("ask_user")
@RequiredArgsConstructor
public class AskUserTool implements Tool {

    private final InteractionService interactionService;

    @Override
    public AskUserResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
