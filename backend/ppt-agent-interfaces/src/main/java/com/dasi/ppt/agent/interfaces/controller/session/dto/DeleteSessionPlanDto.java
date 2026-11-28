package com.dasi.ppt.agent.interfaces.controller.session.dto;

import lombok.Getter;

@Getter
public final class DeleteSessionPlanDto {
    private String sessionId;
    private String expectedRevision;
}
