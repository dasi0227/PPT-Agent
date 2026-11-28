package com.dasi.ppt.agent.domain.agent.service.action.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.service.action.result.HandoffResult;
import com.dasi.ppt.agent.domain.agent.repository.IActionRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.action.Action;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.HandoffPromptAssembler;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.function.BooleanSupplier;

@Component("handoff")
@RequiredArgsConstructor
public class HandoffAction implements Action {

    private final HandoffPromptAssembler handoffPromptAssembler;
    private final ILlmGateway llmGateway;
    private final ISessionRepository sessionRepository;
    private final IActionRepository actionRepository;
    private final DeckService deckService;

    @Override
    public HandoffResult execute(ActionExecution execution, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
