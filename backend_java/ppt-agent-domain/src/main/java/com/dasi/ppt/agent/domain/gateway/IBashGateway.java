package com.dasi.ppt.agent.domain.gateway;

import com.dasi.ppt.agent.domain.project.model.entity.DeckEditCandidate;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.function.BooleanSupplier;

public interface IBashGateway {

    Result execute(String projectId, String command, BooleanSupplier cancelled);

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class Result {
        private int exitCode;
        private String stdout;
        private String stderr;
        private long durationMs;
        private List<DeckEditCandidate> changes;
    }
}
