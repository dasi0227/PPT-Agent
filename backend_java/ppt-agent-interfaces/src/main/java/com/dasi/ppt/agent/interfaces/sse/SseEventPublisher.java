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
    private final ISessionRepository sessionRepository;
    private final Map<String, Set<SseEmitter>> connections = new ConcurrentHashMap<>();

    public SseEmitter subscribe(String sessionId) {
        // 创建对象，每个连接都需要有独立的 SseEmitter
        SseEmitter emitter = new SseEmitter(0L);

        // 登记清理回调
        emitter.onCompletion(() -> remove(sessionId, emitter));
        emitter.onTimeout(() -> remove(sessionId, emitter));
        emitter.onError(error -> remove(sessionId, emitter));

        // 登记订阅关系：一个会话可以与多个浏览器窗口形成连接
        connections.compute(sessionId, (subscribedSessionId, current) -> {
            if (!sessionRepository.exists(subscribedSessionId)) {
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
        // 根据会话 id 拿到所有订阅者
        Set<SseEmitter> subscribers = connections.get(sessionId);
        if (subscribers == null) {
            return;
        }
        // 转换为字符串发送
        String data = objectMapper.writeValueAsString(event);
        for (SseEmitter emitter : subscribers) {
            try {
                emitter.send(SseEmitter.event().name(event.eventName()).data(data));
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

    // 注定关闭连接
    @Override
    public void close(String sessionId) {
        Set<SseEmitter> subscribers = connections.remove(sessionId);
        if (subscribers != null) {
            subscribers.forEach(SseEmitter::complete);
        }
    }

    // 应用程序关闭时，主动断开所有连接
    @EventListener(ContextClosedEvent.class)
    public void shutdown() {
        connections.keySet().forEach(this::close);
    }
}
