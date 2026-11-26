package com.dasi.ppt.agent.infrastructure.persistence.filesystem.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class Mutation {
    private MutationType type;
    private String ownerId;
    private String requestId;
    private String requestHash;
    private String sessionId;
    private String baseRevision;
    private List<FileChange> fileChanges;
}
