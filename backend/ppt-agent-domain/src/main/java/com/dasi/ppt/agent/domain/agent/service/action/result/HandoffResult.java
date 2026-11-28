package com.dasi.ppt.agent.domain.agent.service.action.result;

import com.dasi.ppt.agent.domain.agent.service.action.ActionResult;
import lombok.Builder;
import lombok.Getter;

@Builder
@Getter
public final class HandoffResult implements ActionResult {

    private final String title;
    private final String content;
}
