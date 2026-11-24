package com.dasi.ppt.agent.domain.agent.service.command.handler;

import com.dasi.ppt.agent.domain.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.Command;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.command.CommandHandler;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.SuggestPromptAssembler;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class SuggestCommandHandler implements CommandHandler {

    private final SuggestPromptAssembler suggestPromptAssembler;
    private final ILlmGateway llmGateway;
    private final ISessionRepository sessionRepository;
    private final DeckService deckService;

    @Override
    public CommandType type() {
        return CommandType.SUGGEST;
    }

    @Override
    public JsonNode execute(Command command, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
