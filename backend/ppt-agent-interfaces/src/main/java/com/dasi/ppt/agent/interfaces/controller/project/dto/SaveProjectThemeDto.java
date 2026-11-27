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
public final class SaveProjectThemeDto {
    @NotBlank
    private String projectId;
    @NotBlank
    private String themeId;

    public Project toEntity() {
        return Project.builder()
                .id(projectId)
                .theme(themeId)
                .build();
    }
}
