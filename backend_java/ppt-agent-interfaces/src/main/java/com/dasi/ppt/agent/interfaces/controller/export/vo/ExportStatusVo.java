package com.dasi.ppt.agent.interfaces.controller.export.vo;

import com.dasi.ppt.agent.domain.project.model.enumeration.ExportStatus;

public record ExportStatusVo(
        ExportStatus status,
        int completedSlides,
        int totalSlides,
        String filename,
        Long filesize,
        String errorMessage
) {
}
