package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class MutationLog {
    private MutationLogType type;
    private String ownerId;
    private String requestId;
    private String requestHash;
    private String sessionId;
    private String baseRevision;
    private List<FileChange> files;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class FileChange {
        private String path;
        private byte[] before;
        private byte[] after;
    }
}
