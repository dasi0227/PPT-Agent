package com.dasi.ppt.agent.interfaces.controller.session.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class SessionListVo {
    private List<SessionSummaryVo> items;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class SessionSummaryVo {
        private String id;
        private String title;
        private long updatedAt;
        private RunStatus runStatus;
        private boolean hasActiveCommands;
    }
}
