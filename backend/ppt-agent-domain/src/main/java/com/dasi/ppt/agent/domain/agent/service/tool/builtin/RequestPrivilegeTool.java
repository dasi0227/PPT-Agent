package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.result.RequestPrivilegeResult;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("request_privilege")
@RequiredArgsConstructor
public class RequestPrivilegeTool implements Tool {

    private final InteractionService interactionService;

    @Override
    public RequestPrivilegeResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
