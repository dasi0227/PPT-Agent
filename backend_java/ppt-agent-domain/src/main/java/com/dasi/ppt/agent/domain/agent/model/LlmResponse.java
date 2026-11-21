package com.dasi.ppt.agent.domain.agent.model;

import tools.jackson.databind.JsonNode;

import java.util.List;

public record LlmResponse(
        String text,
        String reasoning,
        List<ToolCall> toolCalls,
        List<Usage> usages,
        JsonNode raw
) {

    public record Usage(
            Long inputTokens,
            Long outputTokens,
            Long totalTokens
    ) {
    }
}
