package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import java.util.List;

public record MutationLog(
        MutationLogType type,
        String ownerId,
        String requestId,
        String requestHash,
        String sessionId,
        String baseRevision,
        List<FileChange> files
) {

    public record FileChange(
            String path,
            byte[] before,
            byte[] after
    ) {
    }
}
