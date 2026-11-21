package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;

import java.util.Set;

public interface AgentTool {

    Set<ToolName> names();

    ToolResult execute(ToolCall call, RunState state);
}
