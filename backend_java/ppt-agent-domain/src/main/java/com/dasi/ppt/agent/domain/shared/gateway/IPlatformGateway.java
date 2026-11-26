package com.dasi.ppt.agent.domain.shared.gateway;

import java.nio.file.Path;

public interface IPlatformGateway {

    Path pickApplication();

    void openFile(String projectId, String fileName, Path application);
}
