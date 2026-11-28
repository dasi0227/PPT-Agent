package com.dasi.ppt.agent.domain.agent.service.tool.editoutline;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class EditOutlineResponse extends ToolResponse {

    private final String content;
}
