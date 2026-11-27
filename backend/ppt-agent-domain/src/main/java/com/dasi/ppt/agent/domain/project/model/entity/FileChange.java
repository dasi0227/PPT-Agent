package com.dasi.ppt.agent.domain.project.model.entity;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public final class FileChange {
    private final String fileName;
    private final byte[] before;
    private final byte[] after;
}
