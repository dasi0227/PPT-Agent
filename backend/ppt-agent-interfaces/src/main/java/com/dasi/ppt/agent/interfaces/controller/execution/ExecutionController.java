package com.dasi.ppt.agent.interfaces.controller.execution;

import com.dasi.ppt.agent.domain.agent.service.action.ActionService;
import com.dasi.ppt.agent.domain.agent.service.run.RunService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class ExecutionController {

    private final RunService runService;
    private final ActionService actionService;
}
