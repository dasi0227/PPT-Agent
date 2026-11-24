package com.dasi.ppt.agent.interfaces.controller.export.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CancelExportDto {
    private String exportId;
}
