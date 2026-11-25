package com.dasi.ppt.agent.domain.project.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Outline {
    private List<Chapter> chapters;

    @Builder
    @Data
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Chapter {
        private String id;
        private String title;
        private List<Page> slides;
        private List<Section> sections;
    }

    @Builder
    @Data
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Section {
        private String id;
        private String title;
        private List<Page> slides;
    }

    @Builder
    @Data
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Page {
        private String id;
        private String title;
    }

    public Set<String> slideIds() {
        Set<String> ids = new LinkedHashSet<>();
        for (Chapter chapter : chapters) {
            chapter.slides.forEach(page -> ids.add(page.id));
            chapter.sections.forEach(section -> section.slides.forEach(page -> ids.add(page.id)));
        }
        return ids;
    }
}
