package com.dasi.ppt.agent.domain.agent.model;

import com.dasi.ppt.agent.domain.agent.model.entity.Command;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.model.entity.Run;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import tools.jackson.databind.JsonNode;

import java.util.List;
import java.util.Map;

public record CheckpointSnapshot(
        String title,
        Plan plan,
        List<Run> runs,
        List<Message> messages,
        List<Command> commands,
        List<AgentReceipt> receipts,
        Map<String, byte[]> files,
        String themeId,
        Map<DecorationType, DecorationPosition> positions,
        byte[] logo,
        JsonNode input
) {

    public record AgentReceipt(
            String runId,
            String toolCallId,
            String requestHash
    ) {
    }
}
