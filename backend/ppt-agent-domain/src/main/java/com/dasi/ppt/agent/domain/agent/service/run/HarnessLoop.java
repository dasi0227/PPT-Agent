package com.dasi.ppt.agent.domain.agent.service.run;

import com.dasi.ppt.agent.domain.shared.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.service.action.builtin.CompactAction;
import com.dasi.ppt.agent.domain.agent.service.context.ContextService;
import com.dasi.ppt.agent.domain.agent.service.hook.HookDispatcher;
import com.dasi.ppt.agent.domain.agent.service.input.InputService;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolExecutor;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class HarnessLoop {

    private final InputService inputService;
    private final RunGuard runGuard;
    private final ContextService contextService;
    private final HookDispatcher hookDispatcher;
    private final ToolExecutor toolExecutor;
    private final CompactAction compactAction;
    private final ILlmGateway llmGateway;
}
