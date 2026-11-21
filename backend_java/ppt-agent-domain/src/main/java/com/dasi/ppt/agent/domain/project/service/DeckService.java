package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.repository.IDeckRepository;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class DeckService {

    private final IDeckRepository deckRepository;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;
}
