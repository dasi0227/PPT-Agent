package com.dasi.ppt.agent.domain.agent.service.hook.afterruncompleted;

import com.dasi.ppt.agent.domain.agent.model.event.RunCompletedEvent;
import com.dasi.ppt.agent.domain.agent.service.command.CommandService;
import com.dasi.ppt.agent.domain.agent.service.hook.AsyncHook;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class AutoSuggestHook implements AsyncHook<RunCompletedEvent> {

    private final CommandService commandService;

    @Override
    public void execute(RunCompletedEvent event) {
        throw new UnsupportedOperationException();
    }
}
