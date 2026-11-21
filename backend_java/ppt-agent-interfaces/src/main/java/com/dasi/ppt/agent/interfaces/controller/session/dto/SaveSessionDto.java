package com.dasi.ppt.agent.interfaces.controller.session.dto;

public record SaveSessionDto(
        String sessionId,
        String projectId,
        String title,
        String expectedRevision
) {
}
