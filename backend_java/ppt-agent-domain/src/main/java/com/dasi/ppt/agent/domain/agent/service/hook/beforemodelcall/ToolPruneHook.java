package com.dasi.ppt.agent.domain.agent.service.hook.beforemodelcall;

import com.dasi.ppt.agent.domain.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.event.BeforeModelCallEvent;
import com.dasi.ppt.agent.domain.agent.model.result.BeforeModelCallResult;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.PrunePromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.hook.SyncHook;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class ToolPruneHook implements SyncHook<BeforeModelCallEvent, BeforeModelCallResult> {

    private final IJevGateway jevGateway;
    private final PrunePromptAssembler prunePromptAssembler;

    @Override
    public BeforeModelCallResult execute(BeforeModelCallEvent event, BeforeModelCallResult currentResult) {
        throw new UnsupportedOperationException();
    }
}
