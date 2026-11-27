package com.dasi.ppt.agent.interfaces.controller.execution.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ActionType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class AppendActionDto {
    private String sessionId;
    private ActionType type;
    private String requestId;
    private String text;
    private String baseId;
    private String feedback;
    private String branchRevision;
}
