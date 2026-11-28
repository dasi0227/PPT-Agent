package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;
import lombok.Getter;

@Getter
public final class CreateProjectDto {
    @NotBlank
    @Size(max = 60, message = "项目名称需为 1–60 个字符")
    private String title;

    public Project toEntity() {
        return Project.builder()
                .title(title)
                .build();
    }
}
