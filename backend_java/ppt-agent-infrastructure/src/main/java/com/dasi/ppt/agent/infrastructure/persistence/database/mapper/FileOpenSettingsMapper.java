package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.FileOpenSettingsPo;

public interface FileOpenSettingsMapper {

    FileOpenSettingsPo read();

    void save(FileOpenSettingsPo settings);
}
