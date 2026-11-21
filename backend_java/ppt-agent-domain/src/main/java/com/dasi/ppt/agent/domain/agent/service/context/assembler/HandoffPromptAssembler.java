package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.agent.model.LlmRequest;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import com.dasi.ppt.agent.domain.project.model.Deck;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;

import java.util.List;

@Component
@RequiredArgsConstructor
public class HandoffPromptAssembler implements PromptAssembler<HandoffPromptAssembler.Input, LlmRequest> {

    private final ResourceLoader resourceLoader;

    @Override
    public LlmRequest assemble(Input input) {
        throw new UnsupportedOperationException();
    }

    public record Input(
            Deck projectContext,
            List<Message> history,
            String lastRevision,
            String revisionFeedback,
            String userInstruction
    ) {
    }
}
