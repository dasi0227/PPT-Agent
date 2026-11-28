package com.dasi.ppt.agent.domain.agent.service.tool.readresource;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReadResourceResponse extends ToolResponse {

    private final String content;
}
