package com.dasi.ppt.agent.domain.agent.service.hook.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeInputProcessedEvent;
import com.dasi.ppt.agent.domain.agent.service.action.ActionExecutor;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.RechristenPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.hook.AsyncHook;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class AutoRenameHook implements AsyncHook<BeforeInputProcessedEvent> {

    private final IJevGateway jevGateway;
    private final RechristenPromptAssembler rechristenPromptAssembler;
    private final ActionExecutor actionExecutor;

    @Override
    public void execute(BeforeInputProcessedEvent event) {
        throw new UnsupportedOperationException();
    }
}
