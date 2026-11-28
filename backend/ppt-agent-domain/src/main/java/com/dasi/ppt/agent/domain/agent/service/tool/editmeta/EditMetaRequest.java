package com.dasi.ppt.agent.domain.agent.service.tool.editmeta;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class EditMetaRequest extends ToolRequest {

    private final String title;
    private final String audience;
    private final String goal;
    private final String language;
    private final String count;
}
