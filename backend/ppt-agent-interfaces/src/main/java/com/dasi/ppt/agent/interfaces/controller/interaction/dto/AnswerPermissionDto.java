package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PermissionDecision;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class AnswerPermissionDto {
    @NotBlank
    private String runId;
    @NotBlank
    private String toolCallId;
    @NotBlank
    private String version;
    @NotNull
    private PermissionDecision decision;
    private String feedback;
}
