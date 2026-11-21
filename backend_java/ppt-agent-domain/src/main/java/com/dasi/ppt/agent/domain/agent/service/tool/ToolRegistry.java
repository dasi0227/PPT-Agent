package com.dasi.ppt.agent.domain.agent.service.tool;

import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.List;

@Component
@RequiredArgsConstructor
public class ToolRegistry {

    private final List<AgentTool> tools;
}
