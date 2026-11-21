package com.dasi.ppt.agent.interfaces.controller.execution.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;

public record AppendCommandDto(
        String sessionId,
        CommandType type,
        String requestId,
        String text,
        String baseId,
        String feedback,
        String branchRevision
) {
}
