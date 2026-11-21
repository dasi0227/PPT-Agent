package com.dasi.ppt.agent.interfaces.controller.project;

import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.project.service.ProjectService;
import com.dasi.ppt.agent.domain.project.service.SlidePreviewService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class ProjectController {

    private final ProjectService projectService;
    private final DeckService deckService;
    private final SlidePreviewService slidePreviewService;
}
