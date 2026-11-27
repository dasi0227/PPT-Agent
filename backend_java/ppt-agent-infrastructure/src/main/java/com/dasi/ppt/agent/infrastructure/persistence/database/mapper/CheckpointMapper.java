package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

public interface CheckpointMapper {

    boolean hasReturnCheckpoint(String sessionId);

    void deleteReturnAndDetachedCheckpoints(String sessionId);
}
