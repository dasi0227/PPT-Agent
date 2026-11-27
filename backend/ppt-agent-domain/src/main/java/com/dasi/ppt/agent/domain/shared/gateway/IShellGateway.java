package com.dasi.ppt.agent.domain.shared.gateway;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.nio.file.Path;
import java.util.function.BooleanSupplier;

public interface IShellGateway {

    CommandResult execute(Path workingDirectory, String command, BooleanSupplier cancellationRequested);

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class CommandResult {
        private int exitCode;
        private String stdout;
        private String stderr;
    }
}
