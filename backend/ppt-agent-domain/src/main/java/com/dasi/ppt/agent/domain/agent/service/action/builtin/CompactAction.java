package com.dasi.ppt.agent.domain.agent.service.action.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ActionExecution;
import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.service.action.result.CompactResult;
import com.dasi.ppt.agent.domain.agent.service.action.Action;
import com.dasi.ppt.agent.domain.agent.service.context.ContextCompactor;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.function.BooleanSupplier;

@Component("compact")
@RequiredArgsConstructor
public class CompactAction implements Action {

    private final ContextCompactor contextCompactor;

    @Override
    public CompactResult execute(ActionExecution execution, RunConfig config, BooleanSupplier cancelled) {
        throw new UnsupportedOperationException();
    }
}
