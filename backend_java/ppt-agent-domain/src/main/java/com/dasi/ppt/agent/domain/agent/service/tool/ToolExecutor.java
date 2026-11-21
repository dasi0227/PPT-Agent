package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class ToolExecutor {

    private final ToolGuard toolGuard;
    private final ToolRegistry toolRegistry;
    private final IRunRepository runRepository;
    private final DeckService deckService;
    private final InteractionService interactionService;
    private final EventPublisher eventPublisher;
}
