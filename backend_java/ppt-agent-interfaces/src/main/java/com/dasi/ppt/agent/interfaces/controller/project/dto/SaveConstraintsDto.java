package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.Constraints;

public record SaveConstraintsDto(
        String projectId,
        String baseHash,
        Constraints constraint
) {
}
