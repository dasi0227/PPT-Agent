package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.entity.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import org.springframework.ai.tool.definition.ToolDefinition;

public interface Tool {

    default ToolDefinition getDefinition(ToolName name) {
        throw new UnsupportedOperationException("尚未实现工具定义：" + name.getValue());
    }

    ToolResult execute(ToolCall call, RunState runState);

}
