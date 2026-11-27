package com.dasi.ppt.agent.interfaces.controller.export.dto;

import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CreateExportDto {
    @NotBlank
    private String projectId;
    @NotNull
    private ExportFormat format;
}
