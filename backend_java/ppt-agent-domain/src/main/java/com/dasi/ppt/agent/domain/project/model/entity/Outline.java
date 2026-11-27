package com.dasi.ppt.agent.domain.project.model.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.AccessLevel;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Outline {
    @JsonProperty("slides")
    private List<Page> pages;
    private List<Chapter> chapters;

    @Builder
    @Data
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Chapter {
        private String id;
        private String title;
        @JsonProperty("slides")
        private List<Page> pages;
        private List<Section> sections;
    }

    @Builder
    @Data
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Section {
        private String id;
        private String title;
        @JsonProperty("slides")
        private List<Page> pages;
    }

    @Builder
    @Data
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Page {
        private String id;
        private String title;
    }

    public PagePosition findPagePosition(String slideId) {
        PagePosition position = PagePosition.builder()
                .build();
        position.countPages(pages, slideId, null);
        for (Chapter chapter : chapters) {
            position.countPages(chapter.pages, slideId, chapter.title);
            for (Section section : chapter.sections) {
                position.countPages(section.pages, slideId, chapter.title);
            }
        }
        return position.pageNumber == 0 ? null : position;
    }

    @Getter
    @Builder(access = AccessLevel.PRIVATE)
    public static final class PagePosition {
        private int pageNumber;
        private int totalPages;
        private String chapterTitle;

        private void countPages(List<Page> pages, String slideId, String chapterTitle) {
            for (Page page : pages) {
                totalPages++;
                if (pageNumber == 0 && slideId.equals(page.id)) {
                    pageNumber = totalPages;
                    this.chapterTitle = chapterTitle;
                }
            }
        }
    }

    public Set<String> slideIds() {
        Set<String> slideIds = new LinkedHashSet<>();
        for (Page page : pages) {
            slideIds.add(page.id);
        }
        for (Chapter chapter : chapters) {
            for (Page page : chapter.pages) {
                slideIds.add(page.id);
            }
            for (Section section : chapter.sections) {
                for (Page page : section.pages) {
                    slideIds.add(page.id);
                }
            }
        }
        return slideIds;
    }

}
