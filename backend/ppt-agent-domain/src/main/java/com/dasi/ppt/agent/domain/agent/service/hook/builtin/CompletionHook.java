package com.dasi.ppt.agent.domain.agent.service.hook.builtin;

import com.dasi.ppt.agent.domain.agent.service.hook.event.BeforeRunCompletedEvent;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeRunCompletedResult;
import com.dasi.ppt.agent.domain.agent.service.hook.SyncHook;
import org.springframework.stereotype.Component;

@Component
public class CompletionHook implements SyncHook<BeforeRunCompletedEvent, BeforeRunCompletedResult> {

    @Override
    public BeforeRunCompletedResult execute(BeforeRunCompletedEvent event, BeforeRunCompletedResult currentResult) {
        throw new UnsupportedOperationException();
    }
}
