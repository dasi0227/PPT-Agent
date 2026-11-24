package com.dasi.ppt.agent.domain.gateway;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

public interface IGitGateway {

    StagedChanges stageAndReadDiff(String projectId);

    String commit(String projectId, String message);

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class StagedChanges {
        private String fileStatus;
        private String lineStatistics;
        private String stagedDiff;
    }
}
