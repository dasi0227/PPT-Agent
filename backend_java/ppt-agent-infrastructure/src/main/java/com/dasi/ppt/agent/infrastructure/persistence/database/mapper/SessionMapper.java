package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import java.util.List;

public interface SessionMapper {

    boolean exists(String sessionId);

    List<String> listProjectSessionIds(String projectId);
}
