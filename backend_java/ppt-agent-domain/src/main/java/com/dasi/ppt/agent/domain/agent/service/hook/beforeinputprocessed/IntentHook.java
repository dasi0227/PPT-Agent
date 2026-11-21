package com.dasi.ppt.agent.domain.agent.service.hook.beforeinputprocessed;

import com.dasi.ppt.agent.domain.agent.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.event.BeforeInputProcessedEvent;
import com.dasi.ppt.agent.domain.agent.model.result.BeforeInputProcessedResult;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.IntentPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.hook.SyncHook;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class IntentHook implements SyncHook<BeforeInputProcessedEvent, BeforeInputProcessedResult> {

    private final IJevGateway jevGateway;
    private final IntentPromptAssembler intentPromptAssembler;

    @Override
    public BeforeInputProcessedResult execute(BeforeInputProcessedEvent event, BeforeInputProcessedResult currentResult) {
        throw new UnsupportedOperationException();
    }
}
