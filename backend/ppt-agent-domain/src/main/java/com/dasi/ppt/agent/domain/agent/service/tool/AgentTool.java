package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import org.springframework.ai.tool.definition.ToolDefinition;

public interface AgentTool {

    ToolName getName();

    default ToolDefinition getDefinition() {
        throw new UnsupportedOperationException("尚未实现工具定义：" + getName().getValue());
    }

    ToolResult execute(ToolCall call, RunState runState);

}
