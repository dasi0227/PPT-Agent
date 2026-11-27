package com.dasi.ppt.agent.domain.agent.service.action.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import com.dasi.ppt.agent.domain.agent.repository.IActionRepository;
import com.dasi.ppt.agent.domain.agent.service.action.Action;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.PolishPromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

@Component("polish")
@RequiredArgsConstructor
public class PolishAction implements Action {

    private final PolishPromptAssembler polishPromptAssembler;
    private final ILlmGateway llmGateway;
    private final IActionRepository actionRepository;

    @Override
    public JsonNode execute(ActionExecution execution, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
