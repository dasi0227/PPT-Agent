package com.dasi.ppt.agent.domain.agent.service.action;

import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;

import java.util.function.BooleanSupplier;

public interface Action {

    ActionResult execute(ActionExecution execution, RunConfig config, BooleanSupplier cancelled);
}
