package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.Outline;

public record SaveOutlineDto(
        String projectId,
        String baseHash,
        Outline outline
) {
}
