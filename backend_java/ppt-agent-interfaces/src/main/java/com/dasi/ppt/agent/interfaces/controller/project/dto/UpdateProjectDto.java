package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.interfaces.controller.StrictRequestDto;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class UpdateProjectDto implements StrictRequestDto {
    @NotBlank
    private String projectId;
    @NotBlank
    private String title;
}
