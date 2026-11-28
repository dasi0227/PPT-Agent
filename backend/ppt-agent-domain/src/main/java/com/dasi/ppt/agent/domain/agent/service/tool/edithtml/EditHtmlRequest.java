package com.dasi.ppt.agent.domain.agent.service.tool.edithtml;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class EditHtmlRequest extends ToolRequest {

    private final String slideId;
    private final String content;
    private final List<Edit> edits;

    @Getter
    @Builder
    public static final class Edit {

        private final String oldText;
        private final String newText;
    }
}
