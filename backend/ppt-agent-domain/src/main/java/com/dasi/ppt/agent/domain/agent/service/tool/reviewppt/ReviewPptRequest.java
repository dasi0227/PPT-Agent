package com.dasi.ppt.agent.domain.agent.service.tool.reviewppt;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReviewPptRequest extends ToolRequest {

    private final String demand;
}
