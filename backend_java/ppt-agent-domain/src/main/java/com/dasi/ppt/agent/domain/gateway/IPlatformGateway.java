package com.dasi.ppt.agent.domain.gateway;

import java.nio.file.Path;

public interface IPlatformGateway {

    Path pickApplication();

    void openFile(String projectId, String relativePath, Path application);
}
