package com.dasi.ppt.agent.domain.agent.service.tool.askuser;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("ask_user")
public class AskUserTool implements Tool<AskUserRequest> {

    @Override
    public ToolResult execute(AskUserRequest request) {
        throw new UnsupportedOperationException();
    }
}
