package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.agent.model.LlmRequest;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;

import java.util.List;

@Component
@RequiredArgsConstructor
public class RenamePromptAssembler implements PromptAssembler<RenamePromptAssembler.Input, LlmRequest> {

    private final ResourceLoader resourceLoader;

    @Override
    public LlmRequest assemble(Input input) {
        throw new UnsupportedOperationException();
    }

    public record Input(
            String currentTitle,
            List<Message> history,
            String userInstruction
    ) {
    }
}
