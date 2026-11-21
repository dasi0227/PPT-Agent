package com.dasi.ppt.agent.domain.agent.service.history;

import com.dasi.ppt.agent.domain.agent.repository.ICheckpointRepository;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.project.service.ProjectWriteLock;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class CheckpointService {

    private final ICheckpointRepository checkpointRepository;
    private final DeckService deckService;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;
}
