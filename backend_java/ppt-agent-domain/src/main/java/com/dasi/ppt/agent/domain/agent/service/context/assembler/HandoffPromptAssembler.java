package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.agent.model.LlmRequest;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import com.dasi.ppt.agent.domain.project.model.entity.Deck;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
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

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Input {
        private Deck projectContext;
        private List<Message> history;
        private String lastRevision;
        private String revisionFeedback;
        private String userInstruction;
    }
}
