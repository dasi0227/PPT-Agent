package com.dasi.ppt.agent.domain.agent.service.tool.reviewppt;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("review_ppt")
public class ReviewPptTool implements Tool<ReviewPptRequest> {

    @Override
    public ToolResponse execute(ReviewPptRequest request) {
        throw new UnsupportedOperationException();
    }
}
