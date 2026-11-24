package com.dasi.ppt.agent.domain.project.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class Outline {
    private List<Section> sections;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Section {
        private String id;
        private String title;
        private List<Page> slides;
        private List<Subsection> subsections;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Subsection {
        private String id;
        private String title;
        private List<Page> slides;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Page {
        private String id;
        private String title;
    }
}
