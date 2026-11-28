package com.dasi.ppt.agent.domain.agent.service.action.result;

import com.dasi.ppt.agent.domain.agent.service.action.ActionResult;
import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
@JsonInclude(JsonInclude.Include.NON_NULL)
public final class CommitResult extends ActionResult {

    private final boolean empty;
    private final String title;
    private final List<String> items;
    private final String hash;
    private final String branch;
    private final Integer filesChanged;
    private final Integer insertions;
    private final Integer deletions;
    private final Long committedAt;
}
