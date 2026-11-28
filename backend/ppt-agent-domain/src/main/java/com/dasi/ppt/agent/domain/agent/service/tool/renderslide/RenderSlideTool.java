package com.dasi.ppt.agent.domain.agent.service.tool.renderslide;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("render_slide")
public class RenderSlideTool implements Tool<RenderSlideRequest> {

    @Override
    public ToolResult execute(RenderSlideRequest request) {
        throw new UnsupportedOperationException();
    }
}
