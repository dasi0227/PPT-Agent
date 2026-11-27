package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
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
