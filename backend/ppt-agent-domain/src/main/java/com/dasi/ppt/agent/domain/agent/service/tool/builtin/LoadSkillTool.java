package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.AgentTool;
import com.dasi.ppt.agent.domain.asset.service.AssetService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("load_skill")
@RequiredArgsConstructor
public class LoadSkillTool implements AgentTool {

    private final AssetService assetService;

    @Override
    public ToolName getName() {
        return ToolName.LOAD_SKILL;
    }

    @Override
    public ToolResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
