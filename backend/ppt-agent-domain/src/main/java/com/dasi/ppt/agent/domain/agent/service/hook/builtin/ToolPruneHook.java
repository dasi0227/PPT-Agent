package com.dasi.ppt.agent.domain.agent.service.hook.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.service.hook.event.BeforeModelCallEvent;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeModelCallResult;
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
