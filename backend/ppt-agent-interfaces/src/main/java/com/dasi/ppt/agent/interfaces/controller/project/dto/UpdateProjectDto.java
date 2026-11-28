package com.dasi.ppt.agent.interfaces.controller.project.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;
import lombok.Getter;

@Getter
public final class UpdateProjectDto {
    @NotBlank
    private String projectId;
    @NotBlank
    @Size(max = 60, message = "项目名称需为 1–60 个字符")
    private String title;
}
