package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.AssetMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.AssetTagMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.TagMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
@RequiredArgsConstructor
public class AssetRepository implements IAssetRepository {

    private final AssetMapper assetMapper;
    private final TagMapper tagMapper;
    private final AssetTagMapper assetTagMapper;

    @Override
    public Optional<Asset> findAsset(String assetId) {
        throw new UnsupportedOperationException();
    }
}
