package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.RouteSettingsPo;
import org.apache.ibatis.annotations.Param;

public interface RouteSettingsMapper {

    RouteSettingsPo getModelRoutes();

    int saveModelRoute(@Param("routeColumn") String routeColumn, @Param("modelId") String modelId);
}
