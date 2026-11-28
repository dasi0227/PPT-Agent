package com.dasi.ppt.agent.domain.agent.service.action.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.IGitGateway;
import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.service.action.result.CommitResult;
import com.dasi.ppt.agent.domain.agent.service.action.Action;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.CommitPromptAssembler;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.function.BooleanSupplier;

@Component("commit")
@RequiredArgsConstructor
public class CommitAction implements Action {

    private final CommitPromptAssembler commitPromptAssembler;
    private final ILlmGateway llmGateway;
    private final IGitGateway gitGateway;

    @Override
    public CommitResult execute(ActionExecution execution, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
