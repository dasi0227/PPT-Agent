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
