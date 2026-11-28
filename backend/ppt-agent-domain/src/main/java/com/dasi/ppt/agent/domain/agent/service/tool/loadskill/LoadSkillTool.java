package com.dasi.ppt.agent.domain.agent.service.tool.loadskill;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("load_skill")
public class LoadSkillTool implements Tool<LoadSkillRequest> {

    @Override
    public ToolResponse execute(LoadSkillRequest request) {
        throw new UnsupportedOperationException();
    }
}
