package com.dasi.ppt.agent.domain.agent.model;

import org.springframework.ai.chat.messages.Message;
import org.springframework.ai.tool.definition.ToolDefinition;

import java.util.List;

public record LlmRequest(
        List<Message> messages,
        List<ToolDefinition> tools
) {
}
