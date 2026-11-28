package com.dasi.ppt.agent.interfaces.controller.project.vo;

import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class ProjectListVo {
    private List<ProjectInfoVo> items;
}
