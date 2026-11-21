package com.dasi.ppt.agent.domain.project.model;

import com.dasi.ppt.agent.domain.project.model.enumeration.ChangeStatus;
import com.dasi.ppt.agent.domain.project.model.enumeration.DiffType;
import tools.jackson.databind.JsonNode;

public record DeckChange(
        ChangeTarget target,
        ChangeStatus status,
        DiffType type,
        JsonNode details
) {
}
