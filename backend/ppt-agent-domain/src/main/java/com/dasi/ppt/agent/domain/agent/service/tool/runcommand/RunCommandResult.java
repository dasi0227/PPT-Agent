package com.dasi.ppt.agent.domain.agent.service.tool.runcommand;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class RunCommandResult extends ToolResult {

    private final Integer exitCode;
    private final String stdout;
    private final String stderr;
}
