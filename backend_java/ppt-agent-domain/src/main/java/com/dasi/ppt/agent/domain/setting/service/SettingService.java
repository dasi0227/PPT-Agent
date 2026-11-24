package com.dasi.ppt.agent.domain.setting.service;

import com.dasi.ppt.agent.domain.setting.model.entity.BehaviorSettings;
import com.dasi.ppt.agent.domain.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelRoute;
import com.dasi.ppt.agent.domain.setting.repository.ISettingRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.nio.file.Path;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Service
@RequiredArgsConstructor
public class SettingService {

    private final ISettingRepository settingRepository;
    private final IPlatformGateway platformGateway;

    public Path pickApplication() {
        return platformGateway.pickApplication();
    }

    public FileOpenSettings getFileOpenSettings() {
        return settingRepository.readFileOpenSettings();
    }

    @Transactional
    public FileOpenSettings saveFileOpenSettings(FileOpenSettings settings) {
        settings.normalize();
        settingRepository.saveFileOpenSettings(settings);
        return settings;
    }

    public ShortcutSettings getShortcuts() {
        return settingRepository.readShortcuts();
    }

    @Transactional
    public void saveShortcuts(ShortcutSettings settings) {
        settings.validate();
        settingRepository.saveShortcuts(settings);
    }

    public BehaviorSettings getBehavior() {
        return settingRepository.readBehaviorSettings();
    }

    @Transactional
    public void saveBehavior(BehaviorSettings settings) {
        settingRepository.saveBehaviorSettings(settings);
    }

    @Transactional(readOnly = true)
    public ModelSettings getModelSettings() {
        return ModelSettings.builder()
                .models(settingRepository.readModels())
                .routes(settingRepository.readRoutes())
                .build();
    }

    @Transactional
    public ModelConfigSettings createModel(ModelConfigSettings config) {
        config.normalize();
        if (settingRepository.modelNameExists(config.getName(), null)) {
            throw invalid("模型配置名不能重复");
        }
        config.setId(IdUtil.genModelId());
        settingRepository.insertModel(config);
        return config;
    }

    @Transactional
    public ModelConfigSettings updateModel(ModelConfigSettings config) {
        settingRepository.requireModel(config.getId());
        config.normalize();
        String name = config.getName();
        if (name != null && settingRepository.modelNameExists(name, config.getId())) {
            throw invalid("模型配置名不能重复");
        }
        return settingRepository.updateModel(config);
    }

    @Transactional
    public void deleteModel(String id) {
        settingRepository.requireModel(id);
        if (settingRepository.countModels() == 1) {
            throw invalid("至少保留一个模型配置");
        }
        if (settingRepository.readRoutes().usesModel(id)) {
            throw conflict(ErrorCode.MODEL_IN_USE);
        }
        settingRepository.deleteModel(id);
    }

    @Transactional
    public void saveRoute(ModelRoute route, String modelId) {
        if (modelId == null && (route == ModelRoute.MAIN_DEFAULT || route == ModelRoute.SIDE_DEFAULT)) {
            throw invalid("默认路由需指定模型配置 ID");
        }
        if (modelId != null) {
            settingRepository.requireModel(modelId);
        }
        settingRepository.saveRoute(route, modelId);
    }
}
