package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.agent.model.LlmRequest;
import com.dasi.ppt.agent.domain.agent.service.context.ContextService;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class AskPromptAssembler implements PromptAssembler<ContextService.Turn, LlmRequest> {

    private final ResourceLoader resourceLoader;

    @Override
    public LlmRequest assemble(ContextService.Turn input) {
        throw new UnsupportedOperationException();
    }
}
