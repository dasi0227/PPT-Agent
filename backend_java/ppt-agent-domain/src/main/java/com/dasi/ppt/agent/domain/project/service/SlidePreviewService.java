package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.agent.gateway.IRenderGateway;
import com.dasi.ppt.agent.domain.project.gateway.IFileGateway;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class SlidePreviewService {

    private final IFileGateway fileGateway;
    private final IRenderGateway renderGateway;
    private final DeckService deckService;
}
