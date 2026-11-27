package com.dasi.ppt.agent.domain.shared.gateway;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

import java.util.Map;
import java.util.function.BooleanSupplier;

public interface IJevGateway {

    JsonNode evaluate(JevRequest jevRequest, BooleanSupplier cancelled);

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class JevRequest {
        private JsonNode payload;
        private Map<String, byte[]> assets;
    }
}
