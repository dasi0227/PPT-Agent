package com.dasi.ppt.agent.interfaces.controller.project.vo;

import java.util.List;

public record ProjectListVo(List<ProjectSummaryVo> items) {

    public record ProjectSummaryVo(
            String id,
            String title,
            long updatedAt
    ) {
    }
}
