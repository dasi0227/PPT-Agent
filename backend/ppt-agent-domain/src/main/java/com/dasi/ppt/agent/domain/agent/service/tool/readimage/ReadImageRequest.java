package com.dasi.ppt.agent.domain.agent.service.tool.readimage;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReadImageRequest extends ToolRequest {

    private final String attachmentId;
    private final String slideId;
}
