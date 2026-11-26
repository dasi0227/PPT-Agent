package com.dasi.ppt.agent.domain.agent.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Checkpoint;

import java.util.List;

public interface ICheckpointRepository {

    List<Checkpoint> listCheckpoints(String sessionId);

    int advanceSessionRevision(String sessionId, long expectedRevision);
}
