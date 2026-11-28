package com.dasi.ppt.agent.domain.agent.service.tool.renderslide;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class RenderSlideRequest extends ToolRequest {

    private final String slideId;
    private final Boolean assessQuality;
}
