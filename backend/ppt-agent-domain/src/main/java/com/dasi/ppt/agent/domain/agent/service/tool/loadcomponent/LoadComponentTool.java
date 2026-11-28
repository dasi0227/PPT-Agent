package com.dasi.ppt.agent.domain.agent.service.tool.loadcomponent;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("load_component")
public class LoadComponentTool implements Tool<LoadComponentRequest> {

    @Override
    public ToolResponse execute(LoadComponentRequest request) {
        throw new UnsupportedOperationException();
    }
}
