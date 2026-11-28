package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.Getter;

@Getter
public final class SaveSlideSpecDto {
    @NotBlank
    private String projectId;
    @NotNull
    @Pattern(regexp = "sli_[A-Za-z0-9_-]+")
    private String slideId;
    private String baseHash;
    @NotNull
    private Spec spec;
}
