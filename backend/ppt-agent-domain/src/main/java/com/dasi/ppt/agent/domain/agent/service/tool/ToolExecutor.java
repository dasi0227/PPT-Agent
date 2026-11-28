package com.dasi.ppt.agent.domain.agent.service.tool;

import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.Map;

@Component
@RequiredArgsConstructor
public class ToolExecutor {

    private final Map<String, Tool<?>> agentTools;

}
