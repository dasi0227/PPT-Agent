package com.dasi.ppt.agent.domain.agent.service.tool.readresource;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("read_resource")
public class ReadResourceTool implements Tool<ReadResourceRequest> {

    @Override
    public ToolResult execute(ReadResourceRequest request) {
        throw new UnsupportedOperationException();
    }
}
