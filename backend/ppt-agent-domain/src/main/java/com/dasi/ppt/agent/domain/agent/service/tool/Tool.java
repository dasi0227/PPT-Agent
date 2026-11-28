package com.dasi.ppt.agent.domain.agent.service.tool;

public interface Tool<R extends ToolRequest> {

    ToolResult execute(R request);
}
