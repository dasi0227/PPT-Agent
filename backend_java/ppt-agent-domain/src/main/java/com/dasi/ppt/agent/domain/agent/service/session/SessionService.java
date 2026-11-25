package com.dasi.ppt.agent.domain.agent.service.session;

import com.dasi.ppt.agent.domain.agent.model.entity.Session;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.project.service.ProjectWriteLock;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.List;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Service
@RequiredArgsConstructor
public class SessionService {

    private final ISessionRepository sessionRepository;
    private final TransactionTemplate transaction;
    private final IProjectRepository projectRepository;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;

    @Transactional(readOnly = true)
    public List<Session> listProjectSessions(String projectId) {
        if (!projectRepository.existsProject(projectId)) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        return sessionRepository.listProjectSessions(projectId);
    }

    public String createSession(Session session) {
        String projectId = session.getProjectId();
        synchronized (projectWriteLock.lockFor(projectId)) {
            List<String> sessionIds = transaction.execute(status -> {
                if (!projectRepository.existsProject(projectId)) {
                    throw notFound(ErrorCode.PROJECT_NOT_FOUND);
                }
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                session.setId(IdUtil.genSessionId());
                if (session.getTitle() == null) {
                    session.setTitle("新会话");
                }
                session.setUpdatedAt(System.currentTimeMillis());
                sessionRepository.insertSession(session);
                return projectRepository.listProjectSessionIds(projectId);
            });
            SseEvent.ProjectChanged event = SseEvent.ProjectChanged.builder()
                    .areas(List.of(ProjectChangeArea.SESSIONS))
                    .build();
            sessionIds.forEach(id -> eventPublisher.publish(id, event));
            return session.getId();
        }
    }
}
