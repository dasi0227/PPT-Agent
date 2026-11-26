package com.dasi.ppt.agent.domain.project.model.entity;

import lombok.Builder;
import lombok.Data;

import java.io.InputStream;

@Data
@Builder
public final class ExportFile {
    private String filename;
    private long size;
    private InputStream content;
}
