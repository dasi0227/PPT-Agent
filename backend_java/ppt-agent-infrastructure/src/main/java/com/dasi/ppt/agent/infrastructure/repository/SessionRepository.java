package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.model.entity.Session;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CheckpointMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CommandMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.MessageMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ReceiptMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.RunMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.SessionMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;
import tools.jackson.databind.ObjectMapper;

import java.util.List;

@Repository
@RequiredArgsConstructor
public class SessionRepository implements ISessionRepository {

    private final SessionMapper sessionMapper;
    private final MessageMapper messageMapper;
    private final RunMapper runMapper;
    private final CommandMapper commandMapper;
    private final CheckpointMapper checkpointMapper;
    private final ReceiptMapper receiptMapper;
    private final ObjectMapper objectMapper;

    @Override
    public Session findSession(String sessionId) {
        throw new UnsupportedOperationException();
    }

    @Override
    public List<Message> listMessages(String sessionId) {
        throw new UnsupportedOperationException();
    }
}
