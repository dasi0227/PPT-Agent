package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
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
