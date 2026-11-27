package com.dasi.ppt.agent.infrastructure.properties;

import lombok.AllArgsConstructor;
import lombok.Getter;
import org.springframework.boot.context.properties.ConfigurationProperties;

import java.nio.file.Path;
import java.time.Duration;
import java.util.List;

@ConfigurationProperties("ppt.sandbox")
@Getter
@AllArgsConstructor
public final class SandboxProperties {
    private final List<Path> allowedFilepaths;
    private final List<String> allowedDomains;
    private final Execution execution;

    @Getter
    @AllArgsConstructor
    public static final class Execution {
        private final Duration timeout;
        private final int maxOutputBytes;
    }
}
