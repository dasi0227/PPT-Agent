package com.dasi.ppt.agent.domain.setting.repository;

import com.dasi.ppt.agent.domain.setting.model.BehaviorSettings;
import com.dasi.ppt.agent.domain.setting.model.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.model.RouteSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.LlmConfig;

import java.util.List;

public interface ISettingRepository {

    FileOpenSettings readFileOpenSettings();

    BehaviorSettings readBehaviorSettings();

    List<LlmConfig> readModels();

    RouteSettings readRoutes();
}
