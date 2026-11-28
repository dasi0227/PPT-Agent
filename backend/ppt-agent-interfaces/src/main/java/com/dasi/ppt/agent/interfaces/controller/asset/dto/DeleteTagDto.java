package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.Getter;

@Getter
public final class DeleteTagDto {
    @NotBlank(message = "缺少 tag_id")
    private String tagId;
}
