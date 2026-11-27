package com.dasi.ppt.agent.domain.agent.service.context;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.CompactPromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class ContextCompactor {

    private final CompactPromptAssembler compactPromptAssembler;
    private final ILlmGateway llmGateway;
    private final ISessionRepository sessionRepository;
    private final IRunRepository runRepository;
}
