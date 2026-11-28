package com.dasi.ppt.agent.domain.agent.service.tool.renderslide;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("render_slide")
public class RenderSlideTool implements Tool<RenderSlideRequest> {

    @Override
    public ToolResponse execute(RenderSlideRequest request) {
        throw new UnsupportedOperationException();
    }
}
