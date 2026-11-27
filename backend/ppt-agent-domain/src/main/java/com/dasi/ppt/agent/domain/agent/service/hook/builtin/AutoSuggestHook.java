package com.dasi.ppt.agent.domain.agent.service.hook.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.RunCompletedEvent;
import com.dasi.ppt.agent.domain.agent.service.action.ActionService;
import com.dasi.ppt.agent.domain.agent.service.hook.AsyncHook;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class AutoSuggestHook implements AsyncHook<RunCompletedEvent> {

    private final ActionService actionService;

    @Override
    public void execute(RunCompletedEvent event) {
        throw new UnsupportedOperationException();
    }
}
