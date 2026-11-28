package com.dasi.ppt.agent.interfaces.controller.project.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.Getter;

@Getter
public final class DeleteProjectDto {
    @NotBlank
    private String projectId;
}
