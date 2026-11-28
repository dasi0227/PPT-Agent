package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.Getter;

@Getter
public final class DeleteAssetDto {
    @NotBlank(message = "缺少 asset_id")
    private String assetId;
}
