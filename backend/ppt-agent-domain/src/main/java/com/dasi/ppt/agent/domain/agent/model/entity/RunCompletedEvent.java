package com.dasi.ppt.agent.domain.agent.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class RunCompletedEvent {
    private String projectId;
    private String sessionId;
    private String runId;
    private String lastInstructionId;
    private RunConfig config;
}
