package com.dasi.ppt.agent.domain.setting.service;

import com.dasi.ppt.agent.domain.setting.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.setting.repository.ISettingRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class SettingService {

    private final ISettingRepository settingRepository;
    private final IPlatformGateway platformGateway;
}
