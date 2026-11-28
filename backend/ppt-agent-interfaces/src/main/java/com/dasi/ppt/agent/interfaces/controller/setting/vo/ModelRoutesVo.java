package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import com.dasi.ppt.agent.domain.setting.model.entity.ModelRouteSettings;
import lombok.Builder;
import lombok.Getter;

@Builder
@Getter
public final class ModelRoutesVo {
    private String mainDefault;
    private String mainFallback;
    private String sideDefault;
    private String sideFallback;
    private String rename;
    private String compact;
    private String commit;
    private String polish;
    private String handoff;
    private String suggest;

    public static ModelRoutesVo toVo(ModelRouteSettings routes) {
        return ModelRoutesVo.builder()
                .mainDefault(routes.getMainDefault())
                .mainFallback(routes.getMainFallback())
                .sideDefault(routes.getSideDefault())
                .sideFallback(routes.getSideFallback())
                .rename(routes.getRename())
                .compact(routes.getCompact())
                .commit(routes.getCommit())
                .polish(routes.getPolish())
                .handoff(routes.getHandoff())
                .suggest(routes.getSuggest())
                .build();
    }
}
