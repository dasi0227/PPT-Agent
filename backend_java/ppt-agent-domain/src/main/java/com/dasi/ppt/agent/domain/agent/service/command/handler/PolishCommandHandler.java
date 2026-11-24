package com.dasi.ppt.agent.domain.agent.service.command.handler;

import com.dasi.ppt.agent.domain.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.Command;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import com.dasi.ppt.agent.domain.agent.repository.ICommandRepository;
import com.dasi.ppt.agent.domain.agent.service.command.CommandHandler;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.PolishPromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class PolishCommandHandler implements CommandHandler {

    private final PolishPromptAssembler polishPromptAssembler;
    private final ILlmGateway llmGateway;
    private final ICommandRepository commandRepository;

    @Override
    public CommandType type() {
        return CommandType.POLISH;
    }

    @Override
    public JsonNode execute(Command command, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
