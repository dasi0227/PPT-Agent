package com.dasi.ppt.agent.domain.agent.service.run;

import com.dasi.ppt.agent.domain.agent.gateway.ILlmGateway;
import com.dasi.ppt.agent.domain.agent.service.command.handler.CompactCommandHandler;
import com.dasi.ppt.agent.domain.agent.service.context.ContextService;
import com.dasi.ppt.agent.domain.agent.service.hook.HookDispatcher;
import com.dasi.ppt.agent.domain.agent.service.input.InputService;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolExecutor;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolRegistry;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class HarnessLoop {

    private final InputService inputService;
    private final RunGuard runGuard;
    private final ContextService contextService;
    private final HookDispatcher hookDispatcher;
    private final ToolRegistry toolRegistry;
    private final ToolExecutor toolExecutor;
    private final CompactCommandHandler compactCommandHandler;
    private final ILlmGateway llmGateway;
}
