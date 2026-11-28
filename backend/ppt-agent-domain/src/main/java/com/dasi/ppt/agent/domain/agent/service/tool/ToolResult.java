package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolResultStatus;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public abstract class ToolResult {

    private final ToolResultStatus status;
    private final String text;
    private final String code;
    private final String correction;

}
