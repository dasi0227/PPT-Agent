package com.dasi.ppt.agent.interfaces.sse;

import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import lombok.RequiredArgsConstructor;
import org.springframework.context.event.ContextClosedEvent;
import org.springframework.context.event.EventListener;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Component;
import org.springframework.web.server.ResponseStatusException;
import org.springframework.web.servlet.mvc.method.annotation.SseEmitter;
import tools.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArraySet;

@Component
@RequiredArgsConstructor
public class SseEventPublisher implements EventPublisher {

    private final ObjectMapper objectMapper;
    private final ISessionRepository sessions;
    private final Map<String, Set<SseEmitter>> connections = new ConcurrentHashMap<>();

    public SseEmitter subscribe(String sessionId) throws IOException {
        SseEmitter emitter = new SseEmitter(0L);
        emitter.onCompletion(() -> remove(sessionId, emitter));
        emitter.onTimeout(() -> remove(sessionId, emitter));
        emitter.onError(error -> remove(sessionId, emitter));
        emitter.send(SseEmitter.event().comment("connected"));
        connections.compute(sessionId, (subscribedSessionId, current) -> {
            if (!sessions.exists(subscribedSessionId)) {
                throw new ResponseStatusException(HttpStatus.NOT_FOUND);
            }
            Set<SseEmitter> subscribers = current == null ? new CopyOnWriteArraySet<>() : current;
            subscribers.add(emitter);
            return subscribers;
        });
        return emitter;
    }

    @Override
    public void publish(String sessionId, SseEvent event) {
        Set<SseEmitter> subscribers = connections.get(sessionId);
        if (subscribers == null) {
            return;
        }
        String name;
        if (event instanceof SseEvent.Changed) {
            name = "changed";
        } else if (event instanceof SseEvent.Reset) {
            name = "reset";
        } else if (event instanceof SseEvent.RunProgress) {
            name = "progress";
        } else if (event instanceof SseEvent.CommandProgress) {
            name = "command_progress";
        } else if (event instanceof SseEvent.InteractionChanged) {
            name = "interaction";
        } else {
            throw new IllegalArgumentException("未知的 SSE 事件");
        }
        String data = objectMapper.writeValueAsString(event);
        for (SseEmitter emitter : subscribers) {
            try {
                emitter.send(SseEmitter.event().name(name).data(data));
            } catch (IOException | IllegalStateException disconnected) {
                remove(sessionId, emitter);
            }
        }
    }

    private void remove(String sessionId, SseEmitter emitter) {
        connections.computeIfPresent(sessionId, (ignored, subscribers) -> {
            subscribers.remove(emitter);
            return subscribers.isEmpty() ? null : subscribers;
        });
    }

    @EventListener(ContextClosedEvent.class)
    public void closeConnections() {
        connections.keySet().forEach(this::close);
    }

    @Override
    public void close(String sessionId) {
        Set<SseEmitter> subscribers = connections.remove(sessionId);
        if (subscribers != null) {
            subscribers.forEach(SseEmitter::complete);
        }
    }
}
