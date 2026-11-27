package com.dasi.ppt.agent.interfaces.controller.project.vo;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ProjectInfoVo {
    private String id;
    private String title;
    private long updatedAt;

    public static ProjectInfoVo toVo(Project project) {
        return ProjectInfoVo.builder()
                .id(project.getId())
                .title(project.getTitle())
                .updatedAt(project.getUpdatedAt())
                .build();
    }
}
