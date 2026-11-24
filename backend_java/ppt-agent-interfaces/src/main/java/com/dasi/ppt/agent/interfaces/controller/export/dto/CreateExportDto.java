package com.dasi.ppt.agent.interfaces.controller.export.dto;

import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CreateExportDto {
    private String projectId;
    private ExportFormat format;
    private String requestId;
}
