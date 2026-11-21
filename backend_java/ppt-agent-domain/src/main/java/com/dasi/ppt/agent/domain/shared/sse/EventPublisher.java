package com.dasi.ppt.agent.domain.shared.sse;

public interface EventPublisher {

    void publish(String sessionId, SseEvent event);
}
