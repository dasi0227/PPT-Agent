package com.dasi.ppt.agent.interfaces.controller.export.dto;

import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;

@Getter
public final class CreateExportDto {
    @NotBlank
    private String projectId;
    @NotNull
    private ExportFormat format;
}
