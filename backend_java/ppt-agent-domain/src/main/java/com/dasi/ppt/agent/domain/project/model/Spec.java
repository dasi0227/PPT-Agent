package com.dasi.ppt.agent.domain.project.model;

import com.dasi.ppt.agent.domain.project.model.enumeration.SlideElementType;
import com.dasi.ppt.agent.domain.project.model.enumeration.SlidePurpose;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class Spec {
    private SlidePurpose purpose;
    private String core;
    private List<Element> elements;
    private String layout;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Element {
        private SlideElementType type;
        private String intent;
    }
}
