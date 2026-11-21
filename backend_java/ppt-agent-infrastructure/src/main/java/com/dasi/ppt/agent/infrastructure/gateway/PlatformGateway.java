package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.setting.gateway.IPlatformGateway;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.PathResolver;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.nio.file.Path;
import java.util.Optional;

@Component
@RequiredArgsConstructor
public class PlatformGateway implements IPlatformGateway {

    private final PathResolver pathResolver;

    @Override
    public Optional<Path> pickApplication() {
        throw new UnsupportedOperationException();
    }

    @Override
    public void open(String projectId, String relativePath, String application) {
        throw new UnsupportedOperationException();
    }
}
