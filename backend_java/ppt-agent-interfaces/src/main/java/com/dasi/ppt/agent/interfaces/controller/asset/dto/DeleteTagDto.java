package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class DeleteTagDto {
    @NotBlank(message = "缺少 tag_id")
    private String tagId;
}
