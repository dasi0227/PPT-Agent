package com.dasi.ppt.agent.domain.agent.service.command;

import com.dasi.ppt.agent.domain.agent.repository.ICommandRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.core.task.AsyncTaskExecutor;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
@RequiredArgsConstructor
public class CommandService {

    private final ICommandRepository commandRepository;
    private final ISessionRepository sessionRepository;
    private final LockService lockService;
    private final SettingService settingService;
    private final List<CommandHandler> handlers;
    private final AsyncTaskExecutor asyncTaskExecutor;
    private final EventPublisher eventPublisher;
}
