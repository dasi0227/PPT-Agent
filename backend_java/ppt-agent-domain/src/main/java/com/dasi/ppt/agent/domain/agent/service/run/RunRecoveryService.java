package com.dasi.ppt.agent.domain.agent.service.run;

import com.dasi.ppt.agent.domain.agent.repository.ICommandRepository;
import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class RunRecoveryService {

    private final IProjectRepository projectRepository;
    private final IRunRepository runRepository;
    private final ICommandRepository commandRepository;
    private final DeckService deckService;
    private final LockService lockService;
}
