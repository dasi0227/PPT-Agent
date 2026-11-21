package com.dasi.ppt.agent.domain.agent.model.event;

import com.dasi.ppt.agent.domain.setting.model.RunConfig;

public record RunCompletedEvent(
        String projectId,
        String sessionId,
        String runId,
        String lastInstructionId,
        RunConfig config
) {
}
