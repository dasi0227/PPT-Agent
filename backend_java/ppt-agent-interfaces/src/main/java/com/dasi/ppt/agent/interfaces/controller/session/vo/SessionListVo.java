package com.dasi.ppt.agent.interfaces.controller.session.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;

import java.util.List;

public record SessionListVo(List<SessionSummaryVo> items) {

    public record SessionSummaryVo(
            String id,
            String title,
            long updatedAt,
            RunStatus runStatus,
            boolean hasActiveCommands
    ) {
    }
}
