package com.dasi.ppt.agent.domain.agent.service.tool.editmeta;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("edit_meta")
public class EditMetaTool implements Tool<EditMetaRequest> {

    @Override
    public ToolResult execute(EditMetaRequest request) {
        throw new UnsupportedOperationException();
    }
}
