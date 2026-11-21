package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.AgentTool;
import com.dasi.ppt.agent.domain.asset.service.AssetService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.Set;

@Component
@RequiredArgsConstructor
public class LoadAssetTool implements AgentTool {

    private final AssetService assetService;

    @Override
    public Set<ToolName> names() {
        return Set.of(ToolName.LOAD_SKILL, ToolName.LOAD_COMPONENT);
    }

    @Override
    public ToolResult execute(ToolCall call, RunState state) {
        throw new UnsupportedOperationException();
    }
}
