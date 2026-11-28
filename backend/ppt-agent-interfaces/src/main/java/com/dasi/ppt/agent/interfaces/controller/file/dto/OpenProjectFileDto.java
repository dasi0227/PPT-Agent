package com.dasi.ppt.agent.interfaces.controller.file.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.Getter;

import static com.dasi.ppt.agent.domain.shared.service.PathService.PROJECT_FILE_NAME_REGEX;

@Getter
public final class OpenProjectFileDto {
    @NotBlank(message = "需指定项目 ID")
    private String projectId;
    @NotNull(message = "需指定文稿文件名")
    @Pattern(regexp = PROJECT_FILE_NAME_REGEX, message = "只能打开项目根目录下的文稿文件")
    private String fileName;
}
