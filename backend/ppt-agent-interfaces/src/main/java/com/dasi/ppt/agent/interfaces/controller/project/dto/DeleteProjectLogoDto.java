package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import jakarta.validation.constraints.NotBlank;
import lombok.Getter;

@Getter
public final class DeleteProjectLogoDto {
    @NotBlank
    private String projectId;

    public Project toEntity() {
        return Project.builder()
                .id(projectId)
                .logo(null)
                .build();
    }
}
