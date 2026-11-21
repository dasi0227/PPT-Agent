package com.dasi.ppt.agent.domain.agent.gateway;

import com.dasi.ppt.agent.domain.agent.model.LlmRequest;
import com.dasi.ppt.agent.domain.agent.model.LlmResponse;
import com.dasi.ppt.agent.domain.setting.model.entity.LlmConfig;

import java.util.function.BooleanSupplier;

public interface ILlmGateway {

    LlmResponse call(LlmRequest request, LlmConfig model, LlmConfig fallback, BooleanSupplier cancelled);
}
