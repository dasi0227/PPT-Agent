package com.dasi.ppt.agent.infrastructure.persistence.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.po.BehaviorSettingsPo;

public interface BehaviorSettingsMapper {

    BehaviorSettingsPo findBehaviorSettings();

    int saveBehaviorSettings(BehaviorSettingsPo settings);
}
