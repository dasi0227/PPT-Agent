package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;

@Getter
public final class SaveOutlineDto {
    @NotBlank
    private String projectId;
    private String baseHash;
    @NotNull
    private Outline outline;
}
