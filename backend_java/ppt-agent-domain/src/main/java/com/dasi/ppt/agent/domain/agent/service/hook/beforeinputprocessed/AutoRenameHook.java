package com.dasi.ppt.agent.domain.agent.service.hook.beforeinputprocessed;

import com.dasi.ppt.agent.domain.agent.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.event.BeforeInputProcessedEvent;
import com.dasi.ppt.agent.domain.agent.service.command.CommandService;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.RechristenPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.hook.AsyncHook;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class AutoRenameHook implements AsyncHook<BeforeInputProcessedEvent> {

    private final IJevGateway jevGateway;
    private final RechristenPromptAssembler rechristenPromptAssembler;
    private final CommandService commandService;

    @Override
    public void execute(BeforeInputProcessedEvent event) {
        throw new UnsupportedOperationException();
    }
}
