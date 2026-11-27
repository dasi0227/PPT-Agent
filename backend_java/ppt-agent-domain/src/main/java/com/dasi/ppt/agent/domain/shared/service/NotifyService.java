package com.dasi.ppt.agent.domain.shared.service;

import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SessionChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
@RequiredArgsConstructor
public class NotifyService {

    private final IProjectRepository projectRepository;
    private final EventPublisher eventPublisher;

    public void notifyProjectChanged(String projectId, ProjectChangeArea area) {
        notifyProjectChanged(projectRepository.listProjectSessionIds(projectId), area);
    }

    public void notifyProjectChanged(List<String> sessionIds, ProjectChangeArea area) {
        SseEvent.ProjectChanged event = SseEvent.ProjectChanged.builder()
                .areas(List.of(area))
                .build();
        sessionIds.forEach(sessionId -> eventPublisher.publish(sessionId, event));
    }

    public void notifySessionChanged(String sessionId, long revision, SessionChangeArea area) {
        eventPublisher.publish(sessionId, SseEvent.SessionChanged.builder()
                .revision(Long.toString(revision))
                .areas(List.of(area))
                .build());
    }
}
