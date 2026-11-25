package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.SessionPo;

import java.util.List;

public interface SessionMapper {

    boolean exists(String sessionId);

    List<String> listProjectSessionIds(String projectId);

    List<SessionPo> listProjectSessions(String projectId);

    void insertSession(SessionPo session);
}
