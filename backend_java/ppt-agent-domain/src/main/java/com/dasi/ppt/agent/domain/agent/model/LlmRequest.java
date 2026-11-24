package com.dasi.ppt.agent.domain.agent.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import org.springframework.ai.chat.messages.Message;
import org.springframework.ai.tool.definition.ToolDefinition;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class LlmRequest {
    private List<Message> messages;
    private List<ToolDefinition> tools;
}
