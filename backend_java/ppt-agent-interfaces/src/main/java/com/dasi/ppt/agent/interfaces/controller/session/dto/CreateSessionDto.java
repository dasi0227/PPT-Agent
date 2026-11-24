package com.dasi.ppt.agent.interfaces.controller.session.dto;

import com.dasi.ppt.agent.interfaces.controller.dto.StrictRequestDto;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CreateSessionDto implements StrictRequestDto {
    @NotBlank
    private String projectId;
    private String title;
}
