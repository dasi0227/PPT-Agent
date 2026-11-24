package com.dasi.ppt.agent.domain.project.model.entity;

import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import com.dasi.ppt.agent.domain.project.model.enumeration.ExportStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ExportTask {
    private String id;
    private String projectId;
    private ExportFormat format;
    private ExportSnapshot snapshot;
    private ExportStatus status;
}
