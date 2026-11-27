package com.dasi.ppt.agent.domain.agent.service.command.handler;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.Command;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import com.dasi.ppt.agent.domain.agent.repository.ICommandRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.command.CommandHandler;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.HandoffPromptAssembler;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class HandoffCommandHandler implements CommandHandler {

    private final HandoffPromptAssembler handoffPromptAssembler;
    private final ILlmGateway llmGateway;
    private final ISessionRepository sessionRepository;
    private final ICommandRepository commandRepository;
    private final DeckService deckService;

    @Override
    public CommandType type() {
        return CommandType.HANDOFF;
    }

    @Override
    public JsonNode execute(Command command, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
