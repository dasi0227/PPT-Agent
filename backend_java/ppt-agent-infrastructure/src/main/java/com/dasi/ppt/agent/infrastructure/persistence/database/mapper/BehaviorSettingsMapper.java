package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.BehaviorSettingsPo;

public interface BehaviorSettingsMapper {

    BehaviorSettingsPo getBehaviorSettings();

    int saveBehaviorSettings(BehaviorSettingsPo settings);
}
