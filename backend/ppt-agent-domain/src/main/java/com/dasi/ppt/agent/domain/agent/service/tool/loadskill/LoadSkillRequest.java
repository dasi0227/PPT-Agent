package com.dasi.ppt.agent.domain.agent.service.tool.loadskill;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class LoadSkillRequest extends ToolRequest {

    private final String skillId;
}
