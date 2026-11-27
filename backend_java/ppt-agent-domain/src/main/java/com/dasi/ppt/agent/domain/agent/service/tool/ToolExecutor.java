package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class ToolExecutor {

    private final ToolGuard toolGuard;
    private final ToolRegistry toolRegistry;
    private final IRunRepository runRepository;
    private final InteractionService interactionService;
}
