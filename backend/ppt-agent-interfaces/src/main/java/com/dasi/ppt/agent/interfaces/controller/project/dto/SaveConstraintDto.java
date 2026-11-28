package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;

@Getter
public final class SaveConstraintDto {
    @NotBlank
    private String projectId;
    private String baseHash;
    @NotNull
    private Constraint constraint;
}
