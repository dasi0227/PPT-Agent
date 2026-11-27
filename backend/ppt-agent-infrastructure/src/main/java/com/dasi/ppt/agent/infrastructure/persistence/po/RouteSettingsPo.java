package com.dasi.ppt.agent.infrastructure.persistence.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Builder
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
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
