package com.dasi.ppt.agent.domain.agent.model.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;

import java.util.Set;

public record BeforeModelCallResult(Set<ToolName> tools) {
}
