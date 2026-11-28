package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.Decision;
import lombok.Getter;

@Getter
public final class AnswerScopeDto {
    private String runId;
    private String toolCallId;
    private String version;
    private Decision decision;
    private String feedback;
}
