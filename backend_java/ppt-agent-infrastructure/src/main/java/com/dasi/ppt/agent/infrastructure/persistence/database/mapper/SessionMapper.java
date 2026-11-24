package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.SessionPo;

import java.util.List;

public interface SessionMapper {

    boolean exists(String id);

    List<SessionPo> listRevisions(String projectId);

    List<String> advanceCheckpointRevisions(String projectId);
}
