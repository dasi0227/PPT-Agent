package com.dasi.ppt.agent.domain.agent.service.run;

import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.history.CheckpointService;
import com.dasi.ppt.agent.domain.agent.service.hook.HookDispatcher;
import com.dasi.ppt.agent.domain.agent.service.input.InputService;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.core.task.AsyncTaskExecutor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class RunService {

    private final IRunRepository runRepository;
    private final ISessionRepository sessionRepository;
    private final InputService inputService;
    private final SettingService settingService;
    private final CheckpointService checkpointService;
    private final LockService lockService;
    private final HarnessLoop harnessLoop;
    private final HookDispatcher hookDispatcher;
    private final InteractionService interactionService;
    private final EventPublisher eventPublisher;
    private final AsyncTaskExecutor asyncTaskExecutor;
}
