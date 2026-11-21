package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionDecision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.MessageType;
import tools.jackson.databind.JsonNode;

public record Message(
        String id,
        String sessionId,
        String runId,
        long seq,
        MessageType type,
        JsonNode content,
        JsonNode raw,
        String toolCallId,
        InstructionDecision instructionDecision,
        InstructionStatus instructionStatus,
        Long contextPosition,
        long createdAt
) {
}
