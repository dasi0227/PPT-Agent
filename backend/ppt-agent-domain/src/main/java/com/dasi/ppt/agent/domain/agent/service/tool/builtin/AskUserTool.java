package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.AgentTool;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("ask_user")
@RequiredArgsConstructor
public class AskUserTool implements AgentTool {

    private final InteractionService interactionService;

    @Override
    public ToolName getName() {
        return ToolName.ASK_USER;
    }

    @Override
    public ToolResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
