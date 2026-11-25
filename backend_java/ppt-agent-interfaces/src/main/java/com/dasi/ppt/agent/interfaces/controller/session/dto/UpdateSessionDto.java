package com.dasi.ppt.agent.interfaces.controller.session.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class UpdateSessionDto {
    @NotBlank
    private String sessionId;
    @NotBlank
    private String title;
    private String expectedRevision;
}
