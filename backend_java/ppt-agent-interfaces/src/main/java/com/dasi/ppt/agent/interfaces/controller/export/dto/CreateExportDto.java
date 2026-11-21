package com.dasi.ppt.agent.interfaces.controller.export.dto;

import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;

public record CreateExportDto(
        String projectId,
        ExportFormat format,
        String requestId
) {
}
