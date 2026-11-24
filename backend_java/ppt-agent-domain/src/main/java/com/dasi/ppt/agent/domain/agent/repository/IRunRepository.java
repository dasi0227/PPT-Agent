package com.dasi.ppt.agent.domain.agent.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Run;

public interface IRunRepository {

    Run findRun(String runId);
}
