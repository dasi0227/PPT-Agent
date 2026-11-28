package com.dasi.ppt.agent.domain.agent.service.tool.editspec;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class EditSpecResult extends ToolResult {

    private final Spec content;
}
