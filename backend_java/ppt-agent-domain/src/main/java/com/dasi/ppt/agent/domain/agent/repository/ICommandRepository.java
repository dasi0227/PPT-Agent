package com.dasi.ppt.agent.domain.agent.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Command;

import java.util.Optional;

public interface ICommandRepository {

    Optional<Command> findCommand(String commandId);
}
