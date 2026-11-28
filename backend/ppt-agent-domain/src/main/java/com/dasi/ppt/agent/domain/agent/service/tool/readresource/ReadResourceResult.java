package com.dasi.ppt.agent.domain.agent.service.tool.readresource;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReadResourceResult extends ToolResult {

    private final String content;
}
