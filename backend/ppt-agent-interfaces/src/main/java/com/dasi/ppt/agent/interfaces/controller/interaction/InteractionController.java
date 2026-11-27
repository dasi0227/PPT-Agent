package com.dasi.ppt.agent.interfaces.controller.interaction;

import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class InteractionController {

    private final InteractionService interactionService;
}
