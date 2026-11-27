package com.dasi.ppt.agent.infrastructure.persistence.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.po.FileOpenSettingsPo;

public interface FileOpenSettingsMapper {

    FileOpenSettingsPo findFileOpenSettings();

    void saveFileOpenSettings(FileOpenSettingsPo settings);
}
