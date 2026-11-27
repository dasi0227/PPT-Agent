package com.dasi.ppt.agent.startup;

import com.dasi.ppt.agent.infrastructure.repository.ProjectRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.SmartInitializingSingleton;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class StartupRecovery implements SmartInitializingSingleton {

    private final ProjectRepository projectRepository;

    @Override
    public void afterSingletonsInstantiated() {
        projectRepository.recoverProjectDeletions();
        projectRepository.recoverPendingFiles();
    }
}
