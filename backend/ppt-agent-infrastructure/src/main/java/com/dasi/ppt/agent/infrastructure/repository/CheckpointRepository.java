package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Checkpoint;
import com.dasi.ppt.agent.domain.agent.repository.ICheckpointRepository;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.CheckpointMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.CommandMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.MessageMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.RunMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.SessionMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;
import tools.jackson.databind.ObjectMapper;

import java.util.List;

@Repository
@RequiredArgsConstructor
public class CheckpointRepository implements ICheckpointRepository {

    private final CheckpointMapper checkpointMapper;
    private final SessionMapper sessionMapper;
    private final MessageMapper messageMapper;
    private final RunMapper runMapper;
    private final CommandMapper commandMapper;
    private final ProjectMapper projectMapper;
    private final ObjectMapper objectMapper;

    @Override
    public boolean hasReturnCheckpoint(String sessionId) {
        return checkpointMapper.hasReturnCheckpoint(sessionId);
    }

    @Override
    public void deleteReturnAndDetachedCheckpoints(String sessionId) {
        checkpointMapper.deleteReturnAndDetachedCheckpoints(sessionId);
    }

    @Override
    public int advanceSessionRevision(String sessionId, long expectedRevision) {
        return sessionMapper.advanceSessionRevision(sessionId, expectedRevision);
    }

    @Override
    public List<Checkpoint> listCheckpoints(String sessionId) {
        throw new UnsupportedOperationException();
    }
}
