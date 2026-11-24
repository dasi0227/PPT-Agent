package com.dasi.ppt.agent.domain.project.model;

import com.dasi.ppt.agent.domain.project.model.entity.Slide;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.Map;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class Deck {
    private Meta meta;
    private Constraints constraint;
    private Outline outline;
    private Map<String, Slide> slides;
}
