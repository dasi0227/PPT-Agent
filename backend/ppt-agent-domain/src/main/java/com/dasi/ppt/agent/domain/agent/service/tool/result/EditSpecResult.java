package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.dasi.ppt.agent.domain.project.model.entity.DeckChange;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class EditSpecResult extends ToolResult {

    private final Spec content;
    private final List<DeckChange> changes;
}
