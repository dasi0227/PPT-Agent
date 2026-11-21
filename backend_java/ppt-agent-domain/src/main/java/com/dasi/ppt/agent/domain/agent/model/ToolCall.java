package com.dasi.ppt.agent.domain.agent.model;

public record ToolCall(
        String toolCallId,
        String name,
        String arguments
) {
}
