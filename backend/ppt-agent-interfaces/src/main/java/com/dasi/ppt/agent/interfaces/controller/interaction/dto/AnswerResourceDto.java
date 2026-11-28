package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.Decision;
import lombok.Getter;
import tools.jackson.databind.JsonNode;

@Getter
public final class AnswerResourceDto {
    private String runId;
    private String toolCallId;
    private String version;
    private Decision decision;
    private JsonNode draft;
    private String feedback;
}
