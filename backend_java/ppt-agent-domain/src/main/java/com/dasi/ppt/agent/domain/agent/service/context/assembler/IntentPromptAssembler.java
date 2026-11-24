package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.shared.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.event.BeforeInputProcessedEvent;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class IntentPromptAssembler implements PromptAssembler<BeforeInputProcessedEvent, IJevGateway.Request> {

    private final ResourceLoader resourceLoader;

    @Override
    public IJevGateway.Request assemble(BeforeInputProcessedEvent input) {
        throw new UnsupportedOperationException();
    }
}
