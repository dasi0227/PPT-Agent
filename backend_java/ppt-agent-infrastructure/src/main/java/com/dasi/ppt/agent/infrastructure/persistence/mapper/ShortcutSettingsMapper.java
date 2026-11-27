package com.dasi.ppt.agent.infrastructure.persistence.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.po.ShortcutSettingsPo;

public interface ShortcutSettingsMapper {

    ShortcutSettingsPo findShortcutSettings();

    void saveShortcutSettings(ShortcutSettingsPo settings);
}
