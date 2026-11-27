package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.SessionPo;

import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface SessionMapper {

    boolean exists(String sessionId);

    String findProjectId(String sessionId);

    List<String> listProjectSessionIds(String projectId);

    SessionPo findSession(String sessionId);

    List<SessionPo> listProjectSessions(String projectId);

    void insertSession(SessionPo session);

    void updateSessionTitle(@Param("sessionId") String sessionId, @Param("title") String title, @Param("updatedAt") long updatedAt);

    int advanceSessionRevision(@Param("sessionId") String sessionId, @Param("expectedRevision") long expectedRevision);
}
