package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.model.entity.PreviewAssessment;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.fasterxml.jackson.annotation.JsonIgnore;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class RenderSlideResult extends ToolResult {

    @JsonIgnore
    private final byte[] screenshot;
    private final String screenshotId;
    private final List<String> diagnostics;
    private final PreviewAssessment preview;
}
