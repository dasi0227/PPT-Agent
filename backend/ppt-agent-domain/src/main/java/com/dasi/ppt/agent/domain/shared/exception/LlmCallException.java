package com.dasi.ppt.agent.domain.shared.exception;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway.LlmResponse;
import lombok.Getter;

import java.util.List;

@Getter
public class LlmCallException extends RuntimeException {

    private final List<LlmResponse.Usage> usages;

    public LlmCallException(String message, Throwable cause, List<LlmResponse.Usage> usages) {
        super(message, cause);
        this.usages = List.copyOf(usages);
    }
}
