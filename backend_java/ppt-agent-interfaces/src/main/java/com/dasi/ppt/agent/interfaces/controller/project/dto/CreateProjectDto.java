package com.dasi.ppt.agent.interfaces.controller.project.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CreateProjectDto {
    @NotBlank
    @Size(max = 60, message = "项目名称需为 1–60 个字符")
    private String title;
}
