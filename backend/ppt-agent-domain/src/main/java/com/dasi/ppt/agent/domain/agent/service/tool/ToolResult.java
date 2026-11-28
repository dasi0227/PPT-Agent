package com.dasi.ppt.agent.domain.agent.service.tool;

import lombok.Builder;
import lombok.Getter;
import org.springframework.ai.chat.messages.part.MessagePart;

import java.util.List;

@Getter
@Builder
public final class ToolResult {

    private final String toolCallId;
    private final String name;
    private final List<MessagePart> messages;
    private final boolean error;
}
