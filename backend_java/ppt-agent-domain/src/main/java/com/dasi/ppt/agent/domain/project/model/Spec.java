package com.dasi.ppt.agent.domain.project.model;

import com.dasi.ppt.agent.domain.project.model.enumeration.SlideElementType;
import com.dasi.ppt.agent.domain.project.model.enumeration.SlidePurpose;

import java.util.List;

public record Spec(
        SlidePurpose purpose,
        String core,
        List<Element> elements,
        String layout
) {

    public record Element(SlideElementType type, String intent) {
    }
}
