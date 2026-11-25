package com.dasi.ppt.agent.domain.setting.repository;

import com.dasi.ppt.agent.domain.setting.model.entity.BehaviorSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelRouteSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelRoute;

import java.util.List;

public interface ISettingRepository {

    FileOpenSettings readFileOpenSettings();

    void saveFileOpenSettings(FileOpenSettings settings);

    ShortcutSettings readShortcuts();

    void saveShortcuts(ShortcutSettings settings);

    BehaviorSettings readBehaviorSettings();

    void saveBehaviorSettings(BehaviorSettings settings);

    List<ModelConfigSettings> readModels();

    ModelConfigSettings readModel(String id);

    ModelRouteSettings readRoutes();

    boolean modelNameExists(String name, String excludeId);

    boolean modelExists(String id);

    int countModels();

    void insertModel(ModelConfigSettings model);

    void updateModel(ModelConfigSettings model);

    void deleteModel(String id);

    void saveRoute(ModelRoute route, String modelId);
}
