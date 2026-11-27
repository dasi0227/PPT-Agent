package com.dasi.ppt.agent.domain.shared.gateway;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.shared.exception.LlmCallException;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import org.springframework.ai.chat.messages.Message;
import org.springframework.ai.tool.definition.ToolDefinition;
import tools.jackson.databind.JsonNode;

import java.util.List;
import java.util.function.BooleanSupplier;

public interface ILlmGateway {

    LlmResponse call(LlmRequest request, ModelConfigSettings primary, ModelConfigSettings fallback, BooleanSupplier cancellationRequested) throws LlmCallException;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class LlmRequest {
        private List<Message> messages;
        private List<ToolDefinition> tools;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class LlmResponse {
        private String modelConfigId;
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
}
