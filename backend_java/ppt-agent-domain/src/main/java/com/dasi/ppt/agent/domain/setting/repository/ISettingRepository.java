package com.dasi.ppt.agent.domain.setting.repository;

import com.dasi.ppt.agent.domain.setting.model.entity.BehaviorSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelRouteSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelRoute;

import java.util.List;

public interface ISettingRepository {

    FileOpenSettings getFileOpenSettings();

    void saveFileOpenSettings(FileOpenSettings settings);

    ShortcutSettings getShortcutSettings();

    void saveShortcutSettings(ShortcutSettings settings);

    BehaviorSettings getBehaviorSettings();

    void saveBehaviorSettings(BehaviorSettings settings);

    List<ModelConfigSettings> listModels();

    ModelConfigSettings findModelById(String modelId);

    ModelRouteSettings getModelRoutes();

    boolean existsModelName(String name, String modelId);

    boolean existsById(String modelId);

    boolean isLastModel(String modelId);

    void insertModel(ModelConfigSettings model);

    int updateModel(ModelConfigSettings model);

    int deleteModel(String modelId);

    void saveModelRoute(ModelRoute route, String modelId);
}
