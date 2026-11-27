package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway.LlmRequest;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.tool.builtin.ReviewPptTool;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class ReviewPromptAssembler implements PromptAssembler<ReviewPptTool.Input, LlmRequest> {

    private final ResourceLoader resourceLoader;

    @Override
    public LlmRequest assemble(ReviewPptTool.Input input) {
        throw new UnsupportedOperationException();
    }
}
