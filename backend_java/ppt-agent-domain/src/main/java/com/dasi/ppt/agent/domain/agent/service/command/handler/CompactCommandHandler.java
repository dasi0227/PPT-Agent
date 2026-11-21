package com.dasi.ppt.agent.domain.agent.service.command.handler;

import com.dasi.ppt.agent.domain.agent.model.entity.Command;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import com.dasi.ppt.agent.domain.agent.service.command.CommandHandler;
import com.dasi.ppt.agent.domain.agent.service.context.ContextCompactor;
import com.dasi.ppt.agent.domain.setting.model.RunConfig;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class CompactCommandHandler implements CommandHandler {

    private final ContextCompactor contextCompactor;

    @Override
    public CommandType type() {
        return CommandType.COMPACT;
    }

    @Override
    public JsonNode execute(Command command, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
