package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.ReviewPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.AgentTool;
import com.dasi.ppt.agent.domain.project.model.Deck;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.Set;

@Component
@RequiredArgsConstructor
public class ReviewPptTool implements AgentTool {

    private final ReviewPromptAssembler reviewPromptAssembler;
    private final ILlmGateway llmGateway;
    private final DeckService deckService;

    @Override
    public Set<ToolName> names() {
        return Set.of(ToolName.REVIEW_PPT);
    }

    @Override
    public ToolResult execute(ToolCall call, RunState state) {
        throw new UnsupportedOperationException();
    }

    public record Input(
            RunState state,
            String requirements,
            Deck deck
    ) {
    }
}
