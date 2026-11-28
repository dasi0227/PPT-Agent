package com.dasi.ppt.agent.domain.agent.service.tool.reviewppt;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReviewPptResponse extends ToolResponse {

    private final String result;
}
