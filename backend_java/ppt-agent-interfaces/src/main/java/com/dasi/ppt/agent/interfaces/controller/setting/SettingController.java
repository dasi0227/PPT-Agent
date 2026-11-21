package com.dasi.ppt.agent.interfaces.controller.setting;

import com.dasi.ppt.agent.domain.setting.service.SettingService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class SettingController {

    private final SettingService settingService;
}
