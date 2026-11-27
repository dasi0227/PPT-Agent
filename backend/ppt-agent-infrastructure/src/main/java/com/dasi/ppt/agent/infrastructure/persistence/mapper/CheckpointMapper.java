package com.dasi.ppt.agent.infrastructure.persistence.mapper;

public interface CheckpointMapper {

    boolean hasReturnCheckpoint(String sessionId);

    void deleteReturnAndDetachedCheckpoints(String sessionId);
}
