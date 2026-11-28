package com.dasi.ppt.agent.domain.agent.service.tool.edithtml;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("edit_html")
public class EditHtmlTool implements Tool<EditHtmlRequest> {

    @Override
    public ToolResponse execute(EditHtmlRequest request) {
        throw new UnsupportedOperationException();
    }
}
