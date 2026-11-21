package com.dasi.ppt.agent.domain.agent.model.event;

import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;

import java.util.List;
import java.util.Set;

public record BeforeModelCallEvent(
        List<Message> history,
        Message instruction,
        Set<ToolName> allowedTools
) {
}
