package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.result.ReadResourceResult;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("read_resource")
@RequiredArgsConstructor
public class ReadResourceTool implements Tool {

    private final DeckService deckService;

    @Override
    public ReadResourceResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
