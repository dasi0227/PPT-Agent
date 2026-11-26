package com.dasi.ppt.agent.interfaces.controller.file.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class OpenProjectFileDto {
    @NotBlank(message = "需指定项目 ID")
    private String projectId;
    @NotBlank(message = "需指定文稿文件名")
    private String fileName;
}
