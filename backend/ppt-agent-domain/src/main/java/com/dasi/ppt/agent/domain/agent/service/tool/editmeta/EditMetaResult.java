package com.dasi.ppt.agent.domain.agent.service.tool.editmeta;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class EditMetaResult extends ToolResult {

    private final Meta content;
}
