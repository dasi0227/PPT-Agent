package com.dasi.ppt.agent.domain.project.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class Constraint {
    private Rules content;
    private Rules visual;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Rules {
        private List<String> requirements;
        private List<String> prohibitions;
    }
}
