package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Command;
import com.dasi.ppt.agent.domain.agent.repository.ICommandRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CheckpointMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CommandMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.MessageMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ReceiptMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.RunMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.SessionMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;
import tools.jackson.databind.ObjectMapper;

@Repository
@RequiredArgsConstructor
public class CommandRepository implements ICommandRepository {

    private final CommandMapper commandMapper;
    private final SessionMapper sessionMapper;
    private final MessageMapper messageMapper;
    private final RunMapper runMapper;
    private final CheckpointMapper checkpointMapper;
    private final ReceiptMapper receiptMapper;
    private final ObjectMapper objectMapper;

    @Override
    public Command findCommand(String commandId) {
        throw new UnsupportedOperationException();
    }
}
