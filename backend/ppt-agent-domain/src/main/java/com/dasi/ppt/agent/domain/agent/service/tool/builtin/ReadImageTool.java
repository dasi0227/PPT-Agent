package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.entity.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.project.service.AttachmentService;
import com.dasi.ppt.agent.domain.project.service.ThumbnailService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component("read_image")
@RequiredArgsConstructor
public class ReadImageTool implements Tool {

    private final AttachmentService attachmentService;
    private final ThumbnailService thumbnailService;

    @Override
    public ToolResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
