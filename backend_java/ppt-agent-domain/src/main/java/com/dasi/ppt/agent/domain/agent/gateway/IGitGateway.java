package com.dasi.ppt.agent.domain.agent.gateway;

public interface IGitGateway {

    StagedChanges stageAndReadDiff(String projectId);

    String commit(String projectId, String message);

    record StagedChanges(
            String fileStatus,
            String lineStatistics,
            String stagedDiff
    ) {
    }
}
