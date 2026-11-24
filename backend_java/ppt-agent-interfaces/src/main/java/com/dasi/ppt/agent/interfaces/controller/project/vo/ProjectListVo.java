package com.dasi.ppt.agent.interfaces.controller.project.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ProjectListVo {
    private List<ProjectSummaryVo> items;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ProjectSummaryVo {
        private String id;
        private String title;
        private long updatedAt;
    }
}
