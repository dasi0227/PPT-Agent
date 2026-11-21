package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class ToolGuard {

    private final DeckService deckService;
}
