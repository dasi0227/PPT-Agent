package com.dasi.ppt.agent.domain.agent.service.action.result;

import com.dasi.ppt.agent.domain.agent.service.action.ActionResult;
import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class SuggestResult extends ActionResult {

    private final List<String> requests;
}
