package com.dasi.ppt.agent.domain.agent.service.tool.runcommand;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class RunCommandResponse extends ToolResponse {

    private final Integer exitCode;
    private final String stdout;
    private final String stderr;
}
