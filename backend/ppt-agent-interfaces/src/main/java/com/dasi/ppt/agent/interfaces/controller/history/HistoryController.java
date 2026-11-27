package com.dasi.ppt.agent.interfaces.controller.history;

import com.dasi.ppt.agent.domain.agent.service.history.HistoryService;
import com.dasi.ppt.agent.interfaces.sse.SseEventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.servlet.mvc.method.annotation.SseEmitter;

@RestController
@RequiredArgsConstructor
public class HistoryController {

    private final HistoryService historyService;
    private final SseEventPublisher sseEventPublisher;

    @GetMapping(value = "/sessions/{sessionId}/events", produces = MediaType.TEXT_EVENT_STREAM_VALUE)
    public SseEmitter subscribe(@PathVariable String sessionId) {
        return sseEventPublisher.subscribe(sessionId);
    }
}
