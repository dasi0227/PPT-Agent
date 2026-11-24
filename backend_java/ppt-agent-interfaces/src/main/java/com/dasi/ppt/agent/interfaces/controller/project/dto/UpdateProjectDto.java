package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.interfaces.controller.dto.StrictRequestDto;
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
public final class UpdateProjectDto implements StrictRequestDto {
    @NotBlank
    private String projectId;
    @NotBlank
    @Size(max = 60)
    private String title;
}
