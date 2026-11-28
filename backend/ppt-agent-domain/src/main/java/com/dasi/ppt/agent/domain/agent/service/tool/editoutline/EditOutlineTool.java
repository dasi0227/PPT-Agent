package com.dasi.ppt.agent.domain.agent.service.tool.editoutline;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("edit_outline")
public class EditOutlineTool implements Tool<EditOutlineRequest> {

    @Override
    public ToolResult execute(EditOutlineRequest request) {
        throw new UnsupportedOperationException();
    }
}
