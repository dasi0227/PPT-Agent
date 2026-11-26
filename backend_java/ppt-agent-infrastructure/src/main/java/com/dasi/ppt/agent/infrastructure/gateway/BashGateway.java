package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IBashGateway;
import com.dasi.ppt.agent.domain.shared.service.PathResolver;
import com.dasi.ppt.agent.infrastructure.properties.SandboxProperties;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class BashGateway implements IBashGateway {

    private final SandboxProperties sandboxProperties;
    private final PathResolver pathResolver;

    @Override
    public Result execute(String projectId, String command, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
