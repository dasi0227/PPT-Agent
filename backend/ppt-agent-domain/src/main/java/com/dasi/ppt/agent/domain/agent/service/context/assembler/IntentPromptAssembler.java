package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.shared.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeInputProcessedEvent;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class IntentPromptAssembler implements PromptAssembler<BeforeInputProcessedEvent, IJevGateway.JevRequest> {

    private final ResourceLoader resourceLoader;

    @Override
    public IJevGateway.JevRequest assemble(BeforeInputProcessedEvent input) {
        throw new UnsupportedOperationException();
    }
}
