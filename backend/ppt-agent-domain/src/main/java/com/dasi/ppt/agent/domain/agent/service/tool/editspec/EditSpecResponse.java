package com.dasi.ppt.agent.domain.agent.service.tool.editspec;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class EditSpecResponse extends ToolResponse {

    private final Spec content;
}
