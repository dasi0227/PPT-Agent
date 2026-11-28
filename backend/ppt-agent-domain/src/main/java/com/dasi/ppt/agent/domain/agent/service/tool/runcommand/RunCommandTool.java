package com.dasi.ppt.agent.domain.agent.service.tool.runcommand;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("run_command")
public class RunCommandTool implements Tool<RunCommandRequest> {

    @Override
    public ToolResponse execute(RunCommandRequest request) {
        throw new UnsupportedOperationException();
    }
}
