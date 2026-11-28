package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.model.entity.RunScope;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.ai.tool.definition.ToolDefinition;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;
import java.util.Set;

@Component
@RequiredArgsConstructor
public class ToolExecutor {

    private final DeckService deckService;
    private final Map<String, Tool> toolsByName;
    private final IRunRepository runRepository;
    private final InteractionService interactionService;

    public List<ToolName> getToolNames(RunMode runMode, RunScope runScope) {
        throw new UnsupportedOperationException();
    }

    public List<ToolDefinition> getToolDefinitions(Set<ToolName> names) {
        return names.stream()
                .sorted()
                .map(name -> getToolByName(name).getDefinition(name))
                .toList();
    }

    private Tool getToolByName(ToolName name) {
        return toolsByName.get(name.getValue());
    }
}
