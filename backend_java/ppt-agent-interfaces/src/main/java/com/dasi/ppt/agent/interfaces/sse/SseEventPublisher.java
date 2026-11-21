package com.dasi.ppt.agent.interfaces.sse;

import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import tools.jackson.databind.ObjectMapper;

@Component
@RequiredArgsConstructor
public class SseEventPublisher implements EventPublisher {

    private final ObjectMapper objectMapper;

    @Override
    public void publish(String sessionId, SseEvent event) {
        throw new UnsupportedOperationException();
    }
}
