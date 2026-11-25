package com.dasi.ppt.agent.interfaces.controller.project.vo;

import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import com.dasi.ppt.agent.domain.project.model.entity.Deck;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.LinkedHashMap;
import java.util.Map;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class DeckInfoVo {
    private Meta meta;
    private Constraint constraint;
    private Outline outline;
    private Map<String, SlideContentVo> slides;
    private DeckHashesVo hashes;

    public static DeckInfoVo toVo(Deck deck) {
        Map<String, SlideContentVo> slides = new LinkedHashMap<>();
        Map<String, SlideHashesVo> hashes = new LinkedHashMap<>();
        deck.getSlides().forEach((id, slide) -> {
            slides.put(id, SlideContentVo.builder()
                    .spec(slide.getSpec())
                    .html(slide.getHtml())
                    .build());
            hashes.put(id, SlideHashesVo.builder()
                    .spec(slide.getSpecHash())
                    .html(slide.getHtmlHash())
                    .build());
        });
        return DeckInfoVo.builder()
                .meta(deck.getMeta())
                .constraint(deck.getConstraint())
                .outline(deck.getOutline())
                .slides(slides)
                .hashes(DeckHashesVo.builder()
                        .meta(deck.getMetaHash())
                        .constraint(deck.getConstraintHash())
                        .outline(deck.getOutlineHash())
                        .slides(hashes)
                        .build())
                .build();
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class SlideContentVo {
        private Spec spec;
        private String html;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class DeckHashesVo {
        private String meta;
        private String constraint;
        private String outline;
        private Map<String, SlideHashesVo> slides;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class SlideHashesVo {
        private String spec;
        private String html;
    }
}
