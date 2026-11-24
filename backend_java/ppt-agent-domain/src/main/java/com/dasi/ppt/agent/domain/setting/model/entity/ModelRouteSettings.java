package com.dasi.ppt.agent.domain.setting.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Arrays;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class ModelRouteSettings {
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

    public boolean usesModel(String id) {
        return Arrays.asList(mainDefault, mainFallback, sideDefault, sideFallback, rename, compact, commit,
                polish, handoff, suggest).contains(id);
    }
}
