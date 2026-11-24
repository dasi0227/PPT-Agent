package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Deck;
import com.dasi.ppt.agent.domain.project.repository.IDeckRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CheckpointMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.MessageMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ReceiptMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.SessionMapper;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.PathResolver;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;
import tools.jackson.databind.ObjectMapper;

@Repository
@RequiredArgsConstructor
public class DeckRepository implements IDeckRepository {

    private final FileStorer fileStorer;
    private final PathResolver pathResolver;
    private final ReceiptMapper receiptMapper;
    private final ProjectMapper projectMapper;
    private final SessionMapper sessionMapper;
    private final CheckpointMapper checkpointMapper;
    private final MessageMapper messageMapper;
    private final ObjectMapper objectMapper;

    @Override
    public Deck readDeck(String projectId) {
        throw new UnsupportedOperationException();
    }
}
