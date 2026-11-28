package com.dasi.ppt.agent.domain.agent.service.tool;

import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public abstract class ToolRequest {

    private final String toolCallId;
    private final ToolContext context;
}
