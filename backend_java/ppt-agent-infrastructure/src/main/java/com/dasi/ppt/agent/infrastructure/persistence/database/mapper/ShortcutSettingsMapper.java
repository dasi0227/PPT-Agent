package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.ShortcutSettingsPo;

public interface ShortcutSettingsMapper {

    ShortcutSettingsPo findShortcutSettings();

    void saveShortcutSettings(ShortcutSettingsPo settings);
}
