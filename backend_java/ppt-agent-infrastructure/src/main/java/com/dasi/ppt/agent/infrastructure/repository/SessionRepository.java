package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.agent.model.Plan;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.model.entity.Session;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CheckpointMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CommandMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.MessageMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.RunMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.SessionMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.SessionPo;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;
import tools.jackson.databind.ObjectMapper;

import java.util.List;

@Repository
@RequiredArgsConstructor
public class SessionRepository implements ISessionRepository {

    private final SessionMapper sessionMapper;
    private final MessageMapper messageMapper;
    private final RunMapper runMapper;
    private final CommandMapper commandMapper;
    private final CheckpointMapper checkpointMapper;
    private final ObjectMapper objectMapper;

    @Override
    public boolean exists(String sessionId) {
        return sessionMapper.exists(sessionId);
    }

    @Override
    public String findProjectId(String sessionId) {
        return sessionMapper.findProjectId(sessionId);
    }

    @Override
    public Session findSession(String sessionId) {
        return toEntity(sessionMapper.findSession(sessionId));
    }

    @Override
    public List<Message> listMessages(String sessionId) {
        throw new UnsupportedOperationException();
    }

    @Override
    public List<Session> listProjectSessions(String projectId) {
        return sessionMapper.listProjectSessions(projectId)
                .stream()
                .map(this::toEntity)
                .toList();
    }

    @Override
    public void insertSession(Session session) {
        sessionMapper.insertSession(toPo(session));
    }

    @Override
    public void updateSessionTitle(String sessionId, String title, long updatedAt) {
        sessionMapper.updateSessionTitle(sessionId, title, updatedAt);
    }

    private static SessionPo toPo(Session session) {
        return SessionPo.builder()
                .id(session.getId())
                .projectId(session.getProjectId())
                .title(session.getTitle())
                .updatedAt(session.getUpdatedAt())
                .build();
    }

    private Session toEntity(SessionPo po) {
        if (po == null) return null;
        return Session.builder()
                .id(po.getId())
                .projectId(po.getProjectId())
                .revision(po.getRevision())
                .plan(po.getPlan() == null ? null : objectMapper.readValue(po.getPlan(), Plan.class))
                .title(po.getTitle())
                .updatedAt(po.getUpdatedAt())
                .runStatus(po.getRunStatus() == null ? null : RunStatus.fromValue(po.getRunStatus()))
                .hasActiveCommands(po.isHasActiveCommands())
                .build();
    }
}
