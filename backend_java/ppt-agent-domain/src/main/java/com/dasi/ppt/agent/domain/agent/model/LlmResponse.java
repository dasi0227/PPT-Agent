package com.dasi.ppt.agent.domain.agent.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class LlmResponse {
    private String text;
    private String reasoning;
    private List<ToolCall> toolCalls;
    private List<Usage> usages;
    private JsonNode raw;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Usage {
        private Long inputTokens;
        private Long outputTokens;
        private Long totalTokens;
    }
}
