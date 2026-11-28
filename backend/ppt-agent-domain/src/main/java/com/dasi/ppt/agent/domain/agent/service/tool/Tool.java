package com.dasi.ppt.agent.domain.agent.service.tool;

public interface Tool<R extends ToolRequest> {

    ToolResponse execute(R request);
}
