package com.dasi.ppt.agent.domain.agent.service.interaction;

import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class InteractionService {

    private final IRunRepository runRepository;
    private final EventPublisher eventPublisher;
}
