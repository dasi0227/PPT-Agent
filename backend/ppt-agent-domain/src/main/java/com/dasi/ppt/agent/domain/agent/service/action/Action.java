package com.dasi.ppt.agent.domain.agent.service.action;

import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

public interface Action {

    JsonNode execute(ActionExecution execution, RunConfig config, BooleanSupplier cancelled);
}
