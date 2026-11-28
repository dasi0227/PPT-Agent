package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReadImageResult extends ToolResult {

    private final byte[] image;
    private final String attachmentId;
    private final String screenshotId;
}
