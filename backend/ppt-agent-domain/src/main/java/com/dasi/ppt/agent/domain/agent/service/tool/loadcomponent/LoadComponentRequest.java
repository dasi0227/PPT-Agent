package com.dasi.ppt.agent.domain.agent.service.tool.loadcomponent;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class LoadComponentRequest extends ToolRequest {

    private final String componentId;
}
