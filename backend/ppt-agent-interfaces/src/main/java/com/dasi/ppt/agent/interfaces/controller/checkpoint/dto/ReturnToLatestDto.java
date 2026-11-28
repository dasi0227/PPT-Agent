package com.dasi.ppt.agent.interfaces.controller.checkpoint.dto;

import lombok.Getter;

@Getter
public final class ReturnToLatestDto {
    private String sessionId;
    private String expectedRevision;
}
