package com.dasi.ppt.agent.domain.agent.service.action.result;

import com.dasi.ppt.agent.domain.agent.service.action.ActionResult;
import com.fasterxml.jackson.annotation.JsonIgnore;
import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class CompactResult extends ActionResult {

    private final String title;
    private final String content;
    private final long beforeTokens;
    private final long afterTokens;
    private final long maxTokens;

    @JsonIgnore
    private final List<String> retainedMessageIds;
}
