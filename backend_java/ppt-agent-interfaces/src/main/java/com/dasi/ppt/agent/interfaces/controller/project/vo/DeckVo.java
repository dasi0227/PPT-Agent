package com.dasi.ppt.agent.interfaces.controller.project.vo;

import com.dasi.ppt.agent.domain.project.model.Constraints;
import com.dasi.ppt.agent.domain.project.model.Meta;
import com.dasi.ppt.agent.domain.project.model.Outline;
import com.dasi.ppt.agent.domain.project.model.Spec;

import java.util.Map;

public record DeckVo(
        Meta meta,
        Constraints constraint,
        Outline outline,
        Map<String, SlideContentVo> slides,
        DeckHashesVo hashes
) {

    public record SlideContentVo(Spec spec, String html) {
    }

    public record DeckHashesVo(
            String meta,
            String constraint,
            String outline,
            Map<String, SlideHashesVo> slides
    ) {
    }

    public record SlideHashesVo(String spec, String html) {
    }
}
