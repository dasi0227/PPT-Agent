package com.dasi.ppt.agent.domain.shared.sse;

import com.dasi.ppt.agent.domain.shared.enumeration.RunActivity;

import java.util.List;

public interface SseEvent {

    record Changed(String revision, List<ChangeArea> areas) implements SseEvent {
    }

    record RunProgress(
            String runId,
            RunActivity activity,
            String commentary
    ) implements SseEvent {
    }

    record InteractionChanged(String runId) implements SseEvent {
    }

    record CommandProgress(String id, int phase) implements SseEvent {
    }

    record Reset() implements SseEvent {
    }
}
