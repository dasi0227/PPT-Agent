package com.dasi.ppt.agent.domain.agent.service.hook;

import com.dasi.ppt.agent.domain.agent.model.entity.BeforeInputProcessedEvent;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeModelCallEvent;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeRunCompletedEvent;
import com.dasi.ppt.agent.domain.agent.model.entity.RunCompletedEvent;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeInputProcessedResult;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeModelCallResult;
import com.dasi.ppt.agent.domain.agent.model.entity.BeforeRunCompletedResult;
import lombok.RequiredArgsConstructor;
import org.springframework.core.task.AsyncTaskExecutor;
import org.springframework.stereotype.Component;

import java.util.List;

@Component
@RequiredArgsConstructor
public class HookDispatcher {

    private final List<SyncHook<BeforeInputProcessedEvent, BeforeInputProcessedResult>> beforeInputHooks;
    private final List<AsyncHook<BeforeInputProcessedEvent>> asyncInputHooks;
    private final List<SyncHook<BeforeModelCallEvent, BeforeModelCallResult>> beforeModelHooks;
    private final List<SyncHook<BeforeRunCompletedEvent, BeforeRunCompletedResult>> beforeCompletionHooks;
    private final List<AsyncHook<RunCompletedEvent>> afterCompletionHooks;
    private final AsyncTaskExecutor asyncTaskExecutor;

    public BeforeInputProcessedResult beforeInputProcessed(BeforeInputProcessedEvent event) {
        throw new UnsupportedOperationException();
    }

    public BeforeModelCallResult beforeModelCall(BeforeModelCallEvent event) {
        throw new UnsupportedOperationException();
    }

    public BeforeRunCompletedResult beforeRunCompleted(BeforeRunCompletedEvent event) {
        throw new UnsupportedOperationException();
    }

    public void afterRunCompleted(RunCompletedEvent event) {
        throw new UnsupportedOperationException();
    }
}
