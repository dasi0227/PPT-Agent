package com.dasi.ppt.agent.domain.project.model;

import java.util.List;

public record Outline(List<Section> sections) {

    public record Section(
            String id,
            String title,
            List<Page> slides,
            List<Subsection> subsections
    ) {
    }

    public record Subsection(
            String id,
            String title,
            List<Page> slides
    ) {
    }

    public record Page(String id, String title) {
    }
}
