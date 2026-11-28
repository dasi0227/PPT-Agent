package com.dasi.ppt.agent.domain.agent.service.action.result;

import com.dasi.ppt.agent.domain.agent.service.action.ActionResult;
import lombok.Builder;
import lombok.Getter;

@Builder
@Getter
public final class RenameResult implements ActionResult {

    private final String title;
}
