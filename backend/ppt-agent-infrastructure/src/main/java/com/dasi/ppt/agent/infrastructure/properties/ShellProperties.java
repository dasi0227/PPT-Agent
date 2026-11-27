package com.dasi.ppt.agent.infrastructure.properties;

import lombok.AllArgsConstructor;
import lombok.Getter;
import org.springframework.boot.context.properties.ConfigurationProperties;

import java.time.Duration;

@ConfigurationProperties("ppt.shell")
@Getter
@AllArgsConstructor
public final class ShellProperties {
    private final Duration timeout;
    private final int maxOutputBytes;
}
