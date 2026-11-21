package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.Spec;

public record SaveSlideSpecDto(
        String projectId,
        String slideId,
        String baseHash,
        Spec spec
) {
}
