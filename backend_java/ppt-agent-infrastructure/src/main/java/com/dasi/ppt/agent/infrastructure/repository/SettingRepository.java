package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.setting.model.BehaviorSettings;
import com.dasi.ppt.agent.domain.setting.model.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.model.RouteSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.LlmConfig;
import com.dasi.ppt.agent.domain.setting.repository.ISettingRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.BehaviorSettingsMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.FileOpenSettingsMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.LlmConfigMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.RouteSettingsMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ShortcutSettingsMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;
import tools.jackson.databind.ObjectMapper;

import java.util.List;

@Repository
@RequiredArgsConstructor
public class SettingRepository implements ISettingRepository {

    private final FileOpenSettingsMapper fileOpenSettingsMapper;
    private final ShortcutSettingsMapper shortcutSettingsMapper;
    private final LlmConfigMapper llmConfigMapper;
    private final RouteSettingsMapper routeSettingsMapper;
    private final BehaviorSettingsMapper behaviorSettingsMapper;
    private final ObjectMapper objectMapper;

    @Override
    public FileOpenSettings readFileOpenSettings() {
        throw new UnsupportedOperationException();
    }

    @Override
    public BehaviorSettings readBehaviorSettings() {
        throw new UnsupportedOperationException();
    }

    @Override
    public List<LlmConfig> readModels() {
        throw new UnsupportedOperationException();
    }

    @Override
    public RouteSettings readRoutes() {
        throw new UnsupportedOperationException();
    }
}
