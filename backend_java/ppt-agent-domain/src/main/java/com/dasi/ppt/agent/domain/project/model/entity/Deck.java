package com.dasi.ppt.agent.domain.project.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Map;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Deck {
    private Meta meta;
    private Constraint constraint;
    private Outline outline;
    private Map<String, Slide> slides;
    private String metaHash;
    private String constraintHash;
    private String outlineHash;
}
