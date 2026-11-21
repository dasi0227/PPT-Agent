package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class RunPo {

    private String id;
    private String sessionId;
    private String mode;
    private String model;
    private String scope;
    private String status;
    private String changes;
    private String error;
    private long inputTokens;
    private long outputTokens;
    private long totalTokens;
    private long callCount;
    private long activeDuration;
    private Long activeSince;
    private long createdAt;
    private long updatedAt;
}
