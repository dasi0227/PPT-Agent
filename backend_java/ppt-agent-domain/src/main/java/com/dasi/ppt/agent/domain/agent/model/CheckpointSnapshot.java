package com.dasi.ppt.agent.domain.agent.model;

import com.dasi.ppt.agent.domain.agent.model.entity.Command;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.model.entity.Run;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

import java.util.List;
import java.util.Map;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CheckpointSnapshot {
    private String title;
    private Plan plan;
    private List<Run> runs;
    private List<Message> messages;
    private List<Command> commands;
    private List<AgentReceipt> receipts;
    private Map<String, byte[]> files;
    private JsonNode input;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class AgentReceipt {
        private String runId;
        private String toolCallId;
        private String requestHash;
    }
}
