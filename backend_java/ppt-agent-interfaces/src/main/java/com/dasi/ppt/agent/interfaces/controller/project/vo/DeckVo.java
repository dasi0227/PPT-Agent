package com.dasi.ppt.agent.interfaces.controller.project.vo;

import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.Map;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class DeckVo {
    private Meta meta;
    private Constraint constraint;
    private Outline outline;
    private Map<String, SlideContentVo> slides;
    private DeckHashesVo hashes;

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
