package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolResultStatus;
import com.dasi.ppt.agent.domain.project.model.entity.DeckChange;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ToolResult {
    private ToolResultStatus status;
    private JsonNode content;
    private String text;
    private String code;
    private String correction;
    private List<DeckChange> changes;
    private JsonNode interaction;
    private PreviewAssessment preview;
}
