package com.dasi.ppt.agent.infrastructure.persistence.filesystem.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class FileChange {
    private String fileName;
    private byte[] before;
    private byte[] after;
}
