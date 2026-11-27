package com.dasi.ppt.agent.domain.shared.sse;

import com.dasi.ppt.agent.domain.agent.model.enumeration.RunActivity;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.annotation.JsonSerialize;

import java.util.List;

public interface SseEvent {

    String eventName();

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class SessionChanged implements SseEvent {
        private String revision;
        private List<SessionChangeArea> areas;

        @Override
        public String eventName() {
            return "session_changed";
        }
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class ProjectChanged implements SseEvent {
        private List<ProjectChangeArea> areas;

        @Override
        public String eventName() {
            return "project_changed";
        }
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class RunProgress implements SseEvent {
        private String runId;
        private RunActivity activity;
        private String commentary;

        @Override
        public String eventName() {
            return "progress";
        }
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class InteractionChanged implements SseEvent {
        private String runId;

        @Override
        public String eventName() {
            return "interaction";
        }
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class ActionProgress implements SseEvent {
        private String id;
        private int phase;

        @Override
        public String eventName() {
            return "command_progress";
        }
    }

    @Builder
    @NoArgsConstructor
    @JsonSerialize
    final class Reset implements SseEvent {
        @Override
        public String eventName() {
            return "reset";
        }
    }
}
