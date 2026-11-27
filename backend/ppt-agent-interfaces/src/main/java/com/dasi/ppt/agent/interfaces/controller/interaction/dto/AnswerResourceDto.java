package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ResourceDecision;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class AnswerResourceDto {
    private String runId;
    private String toolCallId;
    private String version;
    private ResourceDecision decision;
    private JsonNode draft;
    private String feedback;
}
