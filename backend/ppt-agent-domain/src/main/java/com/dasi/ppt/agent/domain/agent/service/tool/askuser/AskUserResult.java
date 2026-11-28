package com.dasi.ppt.agent.domain.agent.service.tool.askuser;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class AskUserResult extends ToolResult {

    private final List<String> answers;
}
