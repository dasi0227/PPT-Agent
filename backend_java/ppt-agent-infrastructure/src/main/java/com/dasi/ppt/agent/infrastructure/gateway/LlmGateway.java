package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.agent.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.LlmRequest;
import com.dasi.ppt.agent.domain.agent.model.LlmResponse;
import com.dasi.ppt.agent.domain.setting.model.entity.LlmConfig;
import lombok.RequiredArgsConstructor;
import org.springframework.ai.anthropic.AnthropicChatModel;
import org.springframework.ai.openai.responses.OpenAiResponsesChatModel;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.stereotype.Component;

import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class LlmGateway implements ILlmGateway {

    private final ObjectProvider<OpenAiResponsesChatModel.Builder> responsesBuilders;
    private final ObjectProvider<AnthropicChatModel.Builder> anthropicBuilders;

    @Override
    public LlmResponse call(LlmRequest request, LlmConfig model, LlmConfig fallback, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
