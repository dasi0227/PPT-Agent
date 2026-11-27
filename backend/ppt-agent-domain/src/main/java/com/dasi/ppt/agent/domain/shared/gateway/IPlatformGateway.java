package com.dasi.ppt.agent.domain.shared.gateway;

import java.nio.file.Path;

public interface IPlatformGateway {

    Path pickApp();

    void openFile(Path path, Path application);

}
