package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IGitGateway;
import org.springframework.stereotype.Component;

@Component
public class GitGateway implements IGitGateway {

    @Override
    public StagedChanges stageAndReadDiff(String projectId) {
        throw new UnsupportedOperationException();
    }

    @Override
    public String commit(String projectId, String message) {
        throw new UnsupportedOperationException();
    }
}
