package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.PreviewPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.AgentTool;
import com.dasi.ppt.agent.domain.project.model.Constraints;
import com.dasi.ppt.agent.domain.project.model.Meta;
import com.dasi.ppt.agent.domain.project.model.Spec;
import com.dasi.ppt.agent.domain.project.service.SlidePreviewService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.Set;

@Component
@RequiredArgsConstructor
public class RenderSlideTool implements AgentTool {

    private final SlidePreviewService slidePreviewService;
    private final IJevGateway jevGateway;
    private final PreviewPromptAssembler previewPromptAssembler;

    @Override
    public Set<ToolName> names() {
        return Set.of(ToolName.RENDER_SLIDE);
    }

    @Override
    public ToolResult execute(ToolCall call, RunState state) {
        throw new UnsupportedOperationException();
    }

    public record PreviewInput(
            Meta meta,
            Constraints constraints,
            Spec spec,
            String html,
            byte[] screenshot
    ) {
    }
}
