package com.dasi.ppt.agent.domain.agent.service.history;

import com.dasi.ppt.agent.domain.agent.repository.IActionRepository;
import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class HistoryService {

    private final ISessionRepository sessionRepository;
    private final IRunRepository runRepository;
    private final IActionRepository actionRepository;
    private final SettingService settingService;
    private final InteractionService interactionService;
}
