package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class RouteSettingsPo {

    private int id;
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
}
