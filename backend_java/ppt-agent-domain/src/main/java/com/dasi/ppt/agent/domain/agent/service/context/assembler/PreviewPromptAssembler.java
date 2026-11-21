package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.agent.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.tool.builtin.RenderSlideTool;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class PreviewPromptAssembler implements PromptAssembler<RenderSlideTool.PreviewInput, IJevGateway.Request> {

    private final ResourceLoader resourceLoader;

    @Override
    public IJevGateway.Request assemble(RenderSlideTool.PreviewInput input) {
        throw new UnsupportedOperationException();
    }
}
