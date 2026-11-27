package com.dasi.ppt.agent.domain.agent.service.action.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.action.Action;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.SuggestPromptAssembler;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

@Component("suggest")
@RequiredArgsConstructor
public class SuggestAction implements Action {

    private final SuggestPromptAssembler suggestPromptAssembler;
    private final ILlmGateway llmGateway;
    private final ISessionRepository sessionRepository;
    private final DeckService deckService;

    @Override
    public JsonNode execute(ActionExecution execution, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
