package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.CheckpointSnapshot;

public record Checkpoint(
        String sessionId,
        String runId,
        CheckpointSnapshot snapshot
) {
}
