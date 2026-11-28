package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public abstract class ToolRequest {

    private final String toolCallId;
    private final RunState runState;
}
