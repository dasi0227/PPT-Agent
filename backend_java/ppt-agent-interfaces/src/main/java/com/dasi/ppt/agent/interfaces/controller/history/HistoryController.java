package com.dasi.ppt.agent.interfaces.controller.history;

import com.dasi.ppt.agent.domain.agent.service.history.HistoryService;
import com.dasi.ppt.agent.interfaces.sse.SseEventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class HistoryController {

    private final HistoryService historyService;
    private final SseEventPublisher sseEventPublisher;
}
