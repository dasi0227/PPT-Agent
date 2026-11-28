package com.dasi.ppt.agent.domain.agent.service.tool.askuser;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class AskUserResponse extends ToolResponse {

    private final List<String> answers;
}
