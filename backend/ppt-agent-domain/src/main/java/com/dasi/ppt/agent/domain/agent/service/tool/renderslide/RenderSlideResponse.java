package com.dasi.ppt.agent.domain.agent.service.tool.renderslide;

import com.dasi.ppt.agent.domain.agent.model.entity.PreviewAssessment;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class RenderSlideResponse extends ToolResponse {

    private final byte[] screenshot;
    private final List<String> diagnostics;
    private final Preview preview;

    @Getter
    @Builder
    public static final class Preview {

        private final String feedback;
        private final PreviewAssessment.Scores scores;
    }
}
