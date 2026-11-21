package com.dasi.ppt.agent.domain.project.model;

import com.dasi.ppt.agent.domain.project.model.entity.Slide;

import java.util.Map;

public record Deck(
        Meta meta,
        Constraints constraint,
        Outline outline,
        Map<String, Slide> slides
) {
}
