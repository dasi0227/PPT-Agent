package com.dasi.ppt.agent.domain.agent.service.tool.readresource;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReadResourceRequest extends ToolRequest {

    private final DeckFileType type;
    private final String slideId;
}
