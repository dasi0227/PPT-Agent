package com.dasi.ppt.agent.interfaces.controller.session;

import com.dasi.ppt.agent.domain.agent.service.session.SessionService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class SessionController {

    private final SessionService sessionService;
}
