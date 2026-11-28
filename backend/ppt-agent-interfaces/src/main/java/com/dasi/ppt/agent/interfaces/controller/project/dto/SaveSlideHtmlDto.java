package com.dasi.ppt.agent.interfaces.controller.project.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.Getter;

@Getter
public final class SaveSlideHtmlDto {
    @NotBlank
    private String projectId;
    @NotNull
    @Pattern(regexp = "sli_[A-Za-z0-9_-]+")
    private String slideId;
    private String baseHash;
    @NotNull
    private String html;
}
