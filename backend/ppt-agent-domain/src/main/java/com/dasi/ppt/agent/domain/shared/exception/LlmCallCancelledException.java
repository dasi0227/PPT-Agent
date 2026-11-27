package com.dasi.ppt.agent.domain.shared.exception;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway.LlmResponse;

import java.util.List;

public class LlmCallCancelledException extends LlmCallException {

    public LlmCallCancelledException(List<LlmResponse.Usage> usages) {
        super("模型调用已取消", null, usages);
    }
}
