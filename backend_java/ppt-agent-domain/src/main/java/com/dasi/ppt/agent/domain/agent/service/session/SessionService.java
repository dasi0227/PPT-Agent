package com.dasi.ppt.agent.domain.agent.service.session;

import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.command.CommandService;
import com.dasi.ppt.agent.domain.agent.service.context.ContextService;
import com.dasi.ppt.agent.domain.agent.service.run.RunService;
import com.dasi.ppt.agent.domain.project.service.ProjectWriteLock;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class SessionService {

    private final ISessionRepository sessionRepository;
    private final RunService runService;
    private final CommandService commandService;
    private final ContextService contextService;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;
}
