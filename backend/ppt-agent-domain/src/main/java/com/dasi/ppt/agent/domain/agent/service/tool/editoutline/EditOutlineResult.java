package com.dasi.ppt.agent.domain.agent.service.tool.editoutline;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class EditOutlineResult extends ToolResult {

    private final String content;
}
