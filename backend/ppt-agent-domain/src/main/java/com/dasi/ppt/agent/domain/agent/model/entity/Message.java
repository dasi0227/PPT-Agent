package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionDecision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.MessageType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Message {
    private String id;
    private String sessionId;
    private String runId;
    private long seq;
    private MessageType type;
    private JsonNode content;
    private JsonNode raw;
    private String toolCallId;
    private InstructionDecision instructionDecision;
    private InstructionStatus instructionStatus;
    private Long contextPosition;
    private long createdAt;
}
