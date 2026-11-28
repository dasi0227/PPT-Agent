package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.result.LoadSkillResult;
import com.dasi.ppt.agent.domain.asset.service.AssetService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("load_skill")
@RequiredArgsConstructor
public class LoadSkillTool implements Tool {

    private final AssetService assetService;

    @Override
    public LoadSkillResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
