package com.dasi.ppt.agent.interfaces.controller.file.dto;

import com.dasi.ppt.agent.interfaces.controller.StrictRequestDto;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class OpenProjectFileDto implements StrictRequestDto {
    @NotBlank(message = "需指定项目 ID")
    private String projectId;
    @NotBlank(message = "需指定项目内的文件路径")
    private String path;
}
