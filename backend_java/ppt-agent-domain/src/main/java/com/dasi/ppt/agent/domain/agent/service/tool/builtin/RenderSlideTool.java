package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.PreviewPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.AgentTool;
import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import com.dasi.ppt.agent.domain.project.service.SlidePreviewService;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
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

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class PreviewInput {
        private Meta meta;
        private Constraint constraint;
        private Spec spec;
        private String html;
        private byte[] screenshot;
    }
}
