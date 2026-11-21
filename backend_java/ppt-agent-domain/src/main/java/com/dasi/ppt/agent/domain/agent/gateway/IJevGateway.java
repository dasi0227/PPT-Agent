package com.dasi.ppt.agent.domain.agent.gateway;

import tools.jackson.databind.JsonNode;

import java.util.Map;
import java.util.function.BooleanSupplier;

public interface IJevGateway {

    JsonNode evaluate(Request request, BooleanSupplier cancelled);

    record Request(JsonNode payload, Map<String, byte[]> assets) {
    }
}
