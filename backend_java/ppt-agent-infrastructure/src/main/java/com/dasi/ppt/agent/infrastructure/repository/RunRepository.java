package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Run;
import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CheckpointMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CommandMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.MessageMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.RunMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.SessionMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;
import tools.jackson.databind.ObjectMapper;

@Repository
@RequiredArgsConstructor
public class RunRepository implements IRunRepository {

    private final RunMapper runMapper;
    private final SessionMapper sessionMapper;
    private final MessageMapper messageMapper;
    private final CheckpointMapper checkpointMapper;
    private final CommandMapper commandMapper;
    private final ObjectMapper objectMapper;

    @Override
    public Run findRun(String runId) {
        throw new UnsupportedOperationException();
    }
}
