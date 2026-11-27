package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.shared.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.event.BeforeModelCallEvent;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class PrunePromptAssembler implements PromptAssembler<BeforeModelCallEvent, IJevGateway.JevRequest> {

    private final ResourceLoader resourceLoader;

    @Override
    public IJevGateway.JevRequest assemble(BeforeModelCallEvent input) {
        throw new UnsupportedOperationException();
    }
}
