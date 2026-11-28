package com.dasi.ppt.agent.domain.agent.service.tool.editoutline;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class EditOutlineRequest extends ToolRequest {

    private final String content;
    private final List<Edit> edits;

    @Getter
    @Builder
    public static final class Edit {

        private final String oldText;
        private final String newText;
    }
}
