package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IJevGateway;
import com.dasi.ppt.agent.infrastructure.properties.JevProperties;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;

import java.net.http.HttpClient;
import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class JevGateway implements IJevGateway {

    private final JevProperties jevProperties;
    private final HttpClient httpClient;
    private final ObjectMapper objectMapper;

    @Override
    public JsonNode evaluate(Request request, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
