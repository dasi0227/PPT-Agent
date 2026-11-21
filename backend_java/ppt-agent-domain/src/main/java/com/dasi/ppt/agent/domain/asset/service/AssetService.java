package com.dasi.ppt.agent.domain.asset.service;

import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class AssetService {

    private final IAssetRepository assetRepository;
}
