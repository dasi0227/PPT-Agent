package com.dasi.ppt.agent.domain.agent.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Run;

import java.util.Optional;

public interface IRunRepository {

    Optional<Run> findRun(String runId);
}
