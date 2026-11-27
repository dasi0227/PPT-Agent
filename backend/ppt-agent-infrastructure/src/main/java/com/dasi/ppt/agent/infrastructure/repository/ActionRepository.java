package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import com.dasi.ppt.agent.domain.agent.repository.IActionRepository;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.CheckpointMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.ActionMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.MessageMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.RunMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.SessionMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;
import tools.jackson.databind.ObjectMapper;

@Repository
@RequiredArgsConstructor
public class ActionRepository implements IActionRepository {

    private final ActionMapper actionMapper;
    private final SessionMapper sessionMapper;
    private final MessageMapper messageMapper;
    private final RunMapper runMapper;
    private final CheckpointMapper checkpointMapper;
    private final ObjectMapper objectMapper;

    @Override
    public ActionExecution findAction(String actionId) {
        throw new UnsupportedOperationException();
    }
}
