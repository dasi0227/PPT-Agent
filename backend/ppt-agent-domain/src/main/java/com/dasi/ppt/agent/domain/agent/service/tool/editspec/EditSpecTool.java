package com.dasi.ppt.agent.domain.agent.service.tool.editspec;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("edit_spec")
public class EditSpecTool implements Tool<EditSpecRequest> {

    @Override
    public ToolResult execute(EditSpecRequest request) {
        throw new UnsupportedOperationException();
    }
}
