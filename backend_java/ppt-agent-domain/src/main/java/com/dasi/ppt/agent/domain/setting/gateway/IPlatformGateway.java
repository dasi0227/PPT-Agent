package com.dasi.ppt.agent.domain.setting.gateway;

import java.nio.file.Path;
import java.util.Optional;

public interface IPlatformGateway {

    Optional<Path> pickApplication();

    void open(String projectId, String relativePath, String application);
}
