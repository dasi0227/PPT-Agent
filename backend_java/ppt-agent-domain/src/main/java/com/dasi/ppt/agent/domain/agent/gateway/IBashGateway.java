package com.dasi.ppt.agent.domain.agent.gateway;

import com.dasi.ppt.agent.domain.project.model.DeckEditCandidate;

import java.util.List;
import java.util.function.BooleanSupplier;

public interface IBashGateway {

    Result execute(String projectId, String command, BooleanSupplier cancelled);

    record Result(
            int exitCode,
            String stdout,
            String stderr,
            long durationMs,
            List<DeckEditCandidate> changes
    ) {
    }
}
