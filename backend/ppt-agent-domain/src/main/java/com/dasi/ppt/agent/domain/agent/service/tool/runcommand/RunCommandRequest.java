package com.dasi.ppt.agent.domain.agent.service.tool.runcommand;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class RunCommandRequest extends ToolRequest {

    private final String command;
}
