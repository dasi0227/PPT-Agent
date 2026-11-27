package com.dasi.ppt.agent.interfaces.controller.session.vo;

import com.dasi.ppt.agent.domain.agent.model.entity.Session;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.fasterxml.jackson.annotation.JsonProperty;
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
        @JsonProperty("has_active_commands")
        private boolean hasActiveActions;

        public static SessionSummaryVo toVo(Session session) {
            return SessionSummaryVo.builder()
                    .id(session.getId())
                    .title(session.getTitle())
                    .updatedAt(session.getUpdatedAt())
                    .runStatus(session.getRunStatus())
                    .hasActiveActions(session.isHasActiveActions())
                    .build();
        }
    }
}
