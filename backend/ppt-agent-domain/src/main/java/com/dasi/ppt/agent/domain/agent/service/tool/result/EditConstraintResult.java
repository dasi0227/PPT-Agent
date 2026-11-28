package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ResourceDecision;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import com.dasi.ppt.agent.domain.project.model.entity.DeckChange;
import lombok.Getter;
import lombok.experimental.SuperBuilder;
import tools.jackson.databind.JsonNode;

import java.util.List;

@Getter
@SuperBuilder
public final class EditConstraintResult extends ToolResult {

    private final Constraint content;
    private final List<DeckChange> changes;
    private final JsonNode request;
    private final ResourceDecision decision;
    private final String feedback;
}
