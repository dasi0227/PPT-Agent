package com.dasi.ppt.agent.domain.agent.service.hook.event;

import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class BeforeInputProcessedEvent {
    private String projectId;
    private String sessionId;
    private String runId;
    private Message instruction;
    private List<Message> history;
    private String sessionTitle;
    private RunConfig config;
}
