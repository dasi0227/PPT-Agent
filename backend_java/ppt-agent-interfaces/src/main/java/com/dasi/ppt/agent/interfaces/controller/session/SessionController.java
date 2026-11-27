package com.dasi.ppt.agent.interfaces.controller.session;

import com.dasi.ppt.agent.domain.agent.service.session.SessionService;
import com.dasi.ppt.agent.interfaces.controller.IdVo;
import com.dasi.ppt.agent.interfaces.controller.session.dto.CreateSessionDto;
import com.dasi.ppt.agent.interfaces.controller.session.dto.UpdateSessionDto;
import com.dasi.ppt.agent.interfaces.controller.session.vo.SessionListVo;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class SessionController {

    private final SessionService sessionService;

    @GetMapping("/projects/{projectId}/sessions")
    public SessionListVo listProjectSessions(@PathVariable String projectId) {
        return SessionListVo.builder()
                .items(sessionService.listProjectSessions(projectId)
                        .stream()
                        .map(SessionListVo.SessionSummaryVo::toVo)
                        .toList())
                .build();
    }

    @PostMapping("/sessions/create")
    public IdVo createSession(@Valid @RequestBody CreateSessionDto request) {
        return IdVo.builder()
                .id(sessionService.createSession(request.toEntity()))
                .build();
    }

    @PostMapping("/sessions/update")
    public void updateSession(@Valid @RequestBody UpdateSessionDto request) {
        sessionService.updateSession(request.getSessionId(), request.getTitle(), request.readExpectedRevision());
    }
}
