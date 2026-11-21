package com.dasi.ppt.agent.domain.agent.service.input;

import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.hook.HookDispatcher;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class InputService {

    private final IRunRepository runRepository;
    private final ISessionRepository sessionRepository;
    private final DeckService deckService;
    private final InteractionService interactionService;
    private final HookDispatcher hookDispatcher;
    private final EventPublisher eventPublisher;
}
