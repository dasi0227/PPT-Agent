package com.dasi.ppt.agent.domain.agent.service.command;

import com.dasi.ppt.agent.domain.agent.model.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.Command;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

public interface CommandHandler {

    CommandType type();

    JsonNode execute(Command command, RunConfig config, BooleanSupplier cancelled);
}
