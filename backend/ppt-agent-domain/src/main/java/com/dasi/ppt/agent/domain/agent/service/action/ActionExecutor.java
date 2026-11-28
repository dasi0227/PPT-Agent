package com.dasi.ppt.agent.domain.agent.service.action;

import com.dasi.ppt.agent.domain.agent.repository.IActionRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.core.task.AsyncTaskExecutor;
import org.springframework.stereotype.Service;

import java.util.Map;

@Service
@RequiredArgsConstructor
public class ActionExecutor {

    private final IActionRepository actionRepository;
    private final ISessionRepository sessionRepository;
    private final LockService lockService;
    private final SettingService settingService;
    private final Map<String, Action> actionsByName;
    private final AsyncTaskExecutor applicationTaskExecutor;
    private final EventPublisher eventPublisher;
}
