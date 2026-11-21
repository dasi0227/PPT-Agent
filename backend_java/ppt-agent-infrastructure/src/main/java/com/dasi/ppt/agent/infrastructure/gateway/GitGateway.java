package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.agent.gateway.IGitGateway;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.PathResolver;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class GitGateway implements IGitGateway {

    private final PathResolver pathResolver;

    @Override
    public StagedChanges stageAndReadDiff(String projectId) {
        throw new UnsupportedOperationException();
    }

    @Override
    public String commit(String projectId, String message) {
        throw new UnsupportedOperationException();
    }
}
