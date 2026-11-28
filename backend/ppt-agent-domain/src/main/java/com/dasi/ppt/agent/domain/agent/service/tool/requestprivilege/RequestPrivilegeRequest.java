package com.dasi.ppt.agent.domain.agent.service.tool.requestprivilege;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class RequestPrivilegeRequest extends ToolRequest {

    private final List<String> slideIds;
    private final String reason;
}
