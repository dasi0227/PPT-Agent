package com.dasi.ppt.agent.domain.agent.service.tool.runcommand;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PermissionDecision;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class RunCommandResult extends ToolResult {

    private final Integer exitCode;
    private final String stdout;
    private final String stderr;
    private final ApprovalRequest request;
    private final PermissionDecision decision;
    private final String feedback;

    @Getter
    @Builder
    public static final class ApprovalRequest {

        private final Permission permission;
    }

    @Getter
    @Builder
    public static final class Permission {

        private final String command;
    }
}
