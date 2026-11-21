package com.dasi.ppt.agent.domain.asset.repository;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;

import java.util.Optional;

public interface IAssetRepository {

    Optional<Asset> findAsset(String assetId);
}
