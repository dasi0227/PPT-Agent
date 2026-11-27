package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.IJevGateway;
import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.entity.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.PreviewPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import com.dasi.ppt.agent.domain.project.service.ThumbnailService;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("render_slide")
@RequiredArgsConstructor
public class RenderSlideTool implements Tool {

    private final ThumbnailService thumbnailService;
    private final IJevGateway jevGateway;
    private final PreviewPromptAssembler previewPromptAssembler;

    @Override
    public ToolResult execute(ToolCall call, RunState runState) {
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
