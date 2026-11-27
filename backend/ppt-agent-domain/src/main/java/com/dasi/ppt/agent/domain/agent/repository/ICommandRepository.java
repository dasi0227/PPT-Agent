package com.dasi.ppt.agent.domain.agent.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Command;

public interface ICommandRepository {

    Command findCommand(String commandId);
}
