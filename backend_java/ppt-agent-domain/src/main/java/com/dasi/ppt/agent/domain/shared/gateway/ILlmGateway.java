package com.dasi.ppt.agent.domain.shared.gateway;

import com.dasi.ppt.agent.domain.agent.model.LlmRequest;
import com.dasi.ppt.agent.domain.agent.model.LlmResponse;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;

import java.util.function.BooleanSupplier;

public interface ILlmGateway {

    LlmResponse call(LlmRequest request, ModelConfigSettings model, ModelConfigSettings fallback, BooleanSupplier cancelled);
}
