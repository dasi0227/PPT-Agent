package com.dasi.ppt.agent.infrastructure.persistence.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Builder
@Getter
@Setter
@AllArgsConstructor
@NoArgsConstructor
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
