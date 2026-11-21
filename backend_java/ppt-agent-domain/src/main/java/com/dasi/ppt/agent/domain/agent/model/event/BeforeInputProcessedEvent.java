package com.dasi.ppt.agent.domain.agent.model.event;

import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.setting.model.RunConfig;

import java.util.List;

public record BeforeInputProcessedEvent(
        String projectId,
        String sessionId,
        String runId,
        Message instruction,
        List<Message> history,
        String sessionTitle,
        RunConfig config
) {
}
