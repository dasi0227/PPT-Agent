package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PlanDecision;
import lombok.Getter;

@Getter
public final class AnswerPlanDto {
    private String runId;
    private String toolCallId;
    private String version;
    private PlanDecision decision;
    private String feedback;
}
