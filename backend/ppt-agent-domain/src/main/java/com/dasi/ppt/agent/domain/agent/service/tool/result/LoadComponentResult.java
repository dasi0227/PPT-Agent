package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class LoadComponentResult extends ToolResult {

    private final String content;
    private final List<LoadedResource> resources;
}
