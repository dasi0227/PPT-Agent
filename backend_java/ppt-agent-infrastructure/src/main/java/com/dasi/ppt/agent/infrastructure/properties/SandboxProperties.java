package com.dasi.ppt.agent.infrastructure.properties;

import org.springframework.boot.context.properties.ConfigurationProperties;

import java.nio.file.Path;
import java.time.Duration;
import java.util.List;

@ConfigurationProperties("ppt.sandbox")
public record SandboxProperties(
        List<Path> allowedFilepaths,
        List<String> allowedDomains,
        Execution execution
) {

    public record Execution(Duration timeout, int maxOutputBytes) {
    }
}
