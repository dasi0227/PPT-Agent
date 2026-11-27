package com.dasi.ppt.agent.domain.agent.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;

public interface IActionRepository {

    ActionExecution findAction(String actionId);
}
