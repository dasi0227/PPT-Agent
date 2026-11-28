package com.dasi.ppt.agent.domain.agent.service.tool.readimage;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("read_image")
public class ReadImageTool implements Tool<ReadImageRequest> {

    @Override
    public ToolResponse execute(ReadImageRequest request) {
        throw new UnsupportedOperationException();
    }
}
