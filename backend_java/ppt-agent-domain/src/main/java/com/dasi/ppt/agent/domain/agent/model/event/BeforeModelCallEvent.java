package com.dasi.ppt.agent.domain.agent.model.event;

import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.Set;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class BeforeModelCallEvent {
    private List<Message> history;
    private Message instruction;
    private Set<ToolName> allowedTools;
}
