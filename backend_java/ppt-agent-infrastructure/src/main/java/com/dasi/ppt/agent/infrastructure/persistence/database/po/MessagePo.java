package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class MessagePo {

    private String id;
    private String sessionId;
    private String runId;
    private long seq;
    private String type;
    private String content;
    private String raw;
    private String toolCallId;
    private String instructionDecision;
    private String instructionStatus;
    private Long contextPosition;
    private long createdAt;
}
