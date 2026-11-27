package com.dasi.ppt.agent.domain.agent.service.action.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.action.Action;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.RenamePromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.JsonNode;

import java.util.function.BooleanSupplier;

@Component("rename")
@RequiredArgsConstructor
public class RenameAction implements Action {

    private final RenamePromptAssembler renamePromptAssembler;
    private final ILlmGateway llmGateway;
    private final ISessionRepository sessionRepository;

    @Override
    public JsonNode execute(ActionExecution execution, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
