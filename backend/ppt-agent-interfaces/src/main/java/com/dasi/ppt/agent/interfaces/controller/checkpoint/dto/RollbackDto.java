package com.dasi.ppt.agent.interfaces.controller.checkpoint.dto;

import lombok.Getter;

@Getter
public final class RollbackDto {
    private String runId;
    private String expectedRevision;
}
