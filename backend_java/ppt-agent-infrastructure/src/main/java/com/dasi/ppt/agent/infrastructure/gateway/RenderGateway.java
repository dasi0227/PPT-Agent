package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IRenderGateway;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.ObjectMapper;

import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class RenderGateway implements IRenderGateway {

    private final ObjectMapper objectMapper;

    @Override
    public Result render(Input input, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
