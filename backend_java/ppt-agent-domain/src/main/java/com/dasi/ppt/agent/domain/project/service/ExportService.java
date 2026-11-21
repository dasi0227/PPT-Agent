package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.agent.gateway.IRenderGateway;
import com.dasi.ppt.agent.domain.asset.service.AssetService;
import com.dasi.ppt.agent.domain.project.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.core.task.AsyncTaskExecutor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class ExportService {

    private final IFileGateway fileGateway;
    private final IRenderGateway renderGateway;
    private final IProjectRepository projectRepository;
    private final DeckService deckService;
    private final AssetService assetService;
    private final ProjectWriteLock projectWriteLock;
    private final AsyncTaskExecutor asyncTaskExecutor;
}
