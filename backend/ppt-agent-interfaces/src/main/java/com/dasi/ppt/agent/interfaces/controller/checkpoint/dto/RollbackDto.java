package com.dasi.ppt.agent.interfaces.controller.checkpoint.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class RollbackDto {
    private String runId;
    private String expectedRevision;
}
