package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelRoute;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.RouteSettingsPo;
import org.apache.ibatis.annotations.Param;

public interface RouteSettingsMapper {

    RouteSettingsPo getModelRoutes();

    int saveModelRoute(@Param("route") ModelRoute route, @Param("modelId") String modelId);
}
