package com.dasi.ppt.agent.domain.project.model.entity;

import com.dasi.ppt.agent.domain.project.model.Spec;

public record Slide(
        String slideId,
        Spec spec,
        String html
) {
}
