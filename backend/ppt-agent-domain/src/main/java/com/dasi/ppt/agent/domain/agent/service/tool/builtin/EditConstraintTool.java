package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.entity.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("edit_constraint")
@RequiredArgsConstructor
public class EditConstraintTool implements Tool {

    private final DeckService deckService;
    private final InteractionService interactionService;

    @Override
    public ToolResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
