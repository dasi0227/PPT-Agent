package com.dasi.ppt.agent.domain.project.model;

import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import com.dasi.ppt.agent.domain.project.model.enumeration.ExportStatus;

public record ExportTask(
        String id,
        String projectId,
        ExportFormat format,
        ExportSnapshot snapshot,
        ExportStatus status
) {
}
