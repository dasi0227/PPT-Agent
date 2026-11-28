package com.dasi.ppt.agent.domain.agent.service.tool.loadskill;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("load_skill")
public class LoadSkillTool implements Tool<LoadSkillRequest> {

    @Override
    public ToolResult execute(LoadSkillRequest request) {
        throw new UnsupportedOperationException();
    }
}
