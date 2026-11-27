package com.dasi.ppt.agent.domain.agent.service.session;

import com.dasi.ppt.agent.domain.agent.model.entity.Session;
import com.dasi.ppt.agent.domain.agent.repository.ICheckpointRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.service.NotifyService;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SessionChangeArea;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.List;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Service
@RequiredArgsConstructor
public class SessionService {

    private final ISessionRepository sessionRepository;
    private final ICheckpointRepository checkpointRepository;
    private final TransactionTemplate transaction;
    private final IProjectRepository projectRepository;
    private final LockService lockService;
    private final NotifyService notifyService;

    @Transactional(readOnly = true)
    public List<Session> listProjectSessions(String projectId) {
        if (!projectRepository.existsProject(projectId)) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        return sessionRepository.listProjectSessions(projectId);
    }

    public String createSession(Session session) {
        String projectId = session.getProjectId();
        synchronized (lockService.projectLock(projectId)) {
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
            notifyService.notifyProjectChanged(sessionIds, ProjectChangeArea.SESSIONS);
            return session.getId();
        }
    }

    public void updateSession(String sessionId, String title, Long expectedRevision) {
        String projectId = sessionRepository.findProjectId(sessionId);
        if (projectId == null) {
            throw notFound(ErrorCode.SESSION_NOT_FOUND);
        }
        synchronized (lockService.projectLock(projectId)) {
            Long checkpointsRevision = transaction.execute(status -> {
                Session session = sessionRepository.findSession(sessionId);
                if (session == null) {
                    throw notFound(ErrorCode.SESSION_NOT_FOUND);
                }
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                long revision = session.getRevision();
                if (expectedRevision != null && expectedRevision != revision) {
                    throw conflict(ErrorCode.STALE_REVISION);
                }

                String normalizedTitle = title.strip();
                boolean titleChanged = !normalizedTitle.equals(session.getTitle().strip());
                boolean discardReturn = titleChanged && checkpointRepository.hasReturnCheckpoint(sessionId);
                if (discardReturn && expectedRevision == null) {
                    throw invalid("回退后修改会话标题需提供 expected_revision");
                }

                sessionRepository.updateSessionTitle(sessionId, titleChanged ? normalizedTitle : session.getTitle(), System.currentTimeMillis());
                if (!titleChanged) return null;
                if (discardReturn) {
                    checkpointRepository.deleteReturnAndDetachedCheckpoints(sessionId);
                }
                if (checkpointRepository.advanceSessionRevision(sessionId, revision) == 0) {
                    throw conflict(ErrorCode.STALE_REVISION);
                }
                return discardReturn ? revision + 1 : null;
            });
            notifyService.notifyProjectChanged(projectId, ProjectChangeArea.SESSIONS);
            if (checkpointsRevision != null) {
                notifyService.notifySessionChanged(sessionId, checkpointsRevision, SessionChangeArea.CHECKPOINTS);
            }
        }
    }
}
