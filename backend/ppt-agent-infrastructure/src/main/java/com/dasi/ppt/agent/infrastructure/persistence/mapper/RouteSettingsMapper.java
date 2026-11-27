package com.dasi.ppt.agent.infrastructure.persistence.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.po.RouteSettingsPo;
import org.apache.ibatis.annotations.Param;

public interface RouteSettingsMapper {

    RouteSettingsPo findModelRoutes();

    int saveModelRoute(@Param("routeColumn") String routeColumn, @Param("modelId") String modelId);
}
