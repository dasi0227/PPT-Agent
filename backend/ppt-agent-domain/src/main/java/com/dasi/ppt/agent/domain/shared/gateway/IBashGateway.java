package com.dasi.ppt.agent.domain.shared.gateway;

import com.dasi.ppt.agent.domain.project.model.entity.FileChange;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.nio.file.Path;
import java.util.List;
import java.util.function.BooleanSupplier;

public interface IBashGateway {

    BashResult execute(Path projectDirectory, String command, boolean writable, BooleanSupplier cancellationRequested);

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class BashResult {
        private int exitCode;
        private String stdout;
        private String stderr;
        private long durationMs;
        private List<FileChange> changes;
    }
}
