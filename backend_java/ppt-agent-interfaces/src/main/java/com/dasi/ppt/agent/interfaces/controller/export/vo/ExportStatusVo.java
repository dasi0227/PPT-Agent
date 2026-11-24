package com.dasi.ppt.agent.interfaces.controller.export.vo;

import com.dasi.ppt.agent.domain.project.model.enumeration.ExportStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ExportStatusVo {
    private ExportStatus status;
    private int completedSlides;
    private int totalSlides;
    private String filename;
    private Long filesize;
    private String errorMessage;
}
