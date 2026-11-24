package com.dasi.ppt.agent.domain.shared.sse;

import com.dasi.ppt.agent.domain.agent.model.enumeration.RunActivity;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.annotation.JsonSerialize;

import java.util.List;

public interface SseEvent {

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class Changed implements SseEvent {
        private String revision;
        private List<ChangeArea> areas;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class RunProgress implements SseEvent {
        private String runId;
        private RunActivity activity;
        private String commentary;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class InteractionChanged implements SseEvent {
        private String runId;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class CommandProgress implements SseEvent {
        private String id;
        private int phase;
    }

    @Builder
    @NoArgsConstructor
    @JsonSerialize
    final class Reset implements SseEvent {
    }
}
