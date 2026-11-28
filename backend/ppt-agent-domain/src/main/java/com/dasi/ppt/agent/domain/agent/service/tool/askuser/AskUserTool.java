package com.dasi.ppt.agent.domain.agent.service.tool.askuser;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("ask_user")
public class AskUserTool implements Tool<AskUserRequest> {

    @Override
    public ToolResponse execute(AskUserRequest request) {
        throw new UnsupportedOperationException();
    }
}
