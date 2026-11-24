package com.dasi.ppt.agent.domain.project.model.entity;

import com.dasi.ppt.agent.domain.project.model.enumeration.ChangeStatus;
import com.dasi.ppt.agent.domain.project.model.enumeration.DiffType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class DeckChange {
    private ChangeTarget target;
    private ChangeStatus status;
    private DiffType type;
    private JsonNode details;
}
