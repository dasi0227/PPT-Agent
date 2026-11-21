package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.Meta;

public record SaveMetaDto(
        String projectId,
        String baseHash,
        Meta meta
) {
}
