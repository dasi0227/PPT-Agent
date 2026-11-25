package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.FileOpenSettingsPo;

public interface FileOpenSettingsMapper {

    FileOpenSettingsPo getFileOpenSettings();

    void saveFileOpenSettings(FileOpenSettingsPo settings);
}
