package com.dasi.ppt.agent.domain.agent.service.tool.readimage;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class ReadImageResponse extends ToolResponse {

    private final byte[] image;
}
