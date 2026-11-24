package com.dasi.ppt.agent.infrastructure.persistence.database.po;

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
public class CommandPo {

    private String id;
    private String operationId;
    private String sessionId;
    private String runId;
    private long seq;
    private String type;
    private String status;
    private String input;
    private String result;
    private String error;
    private long createdAt;
}
