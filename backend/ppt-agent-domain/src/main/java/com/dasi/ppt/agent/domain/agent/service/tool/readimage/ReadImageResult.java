package com.dasi.ppt.agent.domain.agent.service.tool.readimage;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReadImageResult extends ToolResult {

    private final byte[] image;
}
