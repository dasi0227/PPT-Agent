package com.dasi.ppt.agent.domain.agent.service.tool.requestprivilege;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("request_privilege")
public class RequestPrivilegeTool implements Tool<RequestPrivilegeRequest> {

    @Override
    public ToolResult execute(RequestPrivilegeRequest request) {
        throw new UnsupportedOperationException();
    }
}
