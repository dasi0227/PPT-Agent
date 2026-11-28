package com.dasi.ppt.agent.domain.agent.service.tool.loadcomponent;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("load_component")
public class LoadComponentTool implements Tool<LoadComponentRequest> {

    @Override
    public ToolResult execute(LoadComponentRequest request) {
        throw new UnsupportedOperationException();
    }
}
