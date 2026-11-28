package com.dasi.ppt.agent.domain.agent.service.tool.editmeta;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("edit_meta")
public class EditMetaTool implements Tool<EditMetaRequest> {

    @Override
    public ToolResponse execute(EditMetaRequest request) {
        throw new UnsupportedOperationException();
    }
}
