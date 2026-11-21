package com.dasi.ppt.agent.interfaces.controller.asset;

import com.dasi.ppt.agent.domain.asset.service.AssetService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class AssetController {

    private final AssetService assetService;
}
