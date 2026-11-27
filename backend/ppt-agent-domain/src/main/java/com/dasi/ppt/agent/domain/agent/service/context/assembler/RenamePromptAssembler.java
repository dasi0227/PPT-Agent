package com.dasi.ppt.agent.domain.agent.service.context.assembler;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway.LlmRequest;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.service.context.PromptAssembler;
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
public class RenamePromptAssembler implements PromptAssembler<RenamePromptAssembler.Input, LlmRequest> {

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
        private String currentTitle;
        private List<Message> history;
        private String userInstruction;
    }
}
