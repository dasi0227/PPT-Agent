package com.dasi.ppt.agent.domain.agent.model.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolResultStatus;
import com.dasi.ppt.agent.domain.project.model.DeckChange;
import tools.jackson.databind.JsonNode;

import java.util.List;

public record ToolResult(
        ToolResultStatus status,
        JsonNode content,
        String text,
        String code,
        String correction,
        List<DeckChange> changes,
        JsonNode interaction,
        PreviewAssessment preview
) {
}
