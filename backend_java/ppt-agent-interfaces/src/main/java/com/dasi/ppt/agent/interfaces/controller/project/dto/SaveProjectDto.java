package com.dasi.ppt.agent.interfaces.controller.project.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record SaveProjectDto(String projectId, @NotBlank @Size(max = 60) String title) {
}
