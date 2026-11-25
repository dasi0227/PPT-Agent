package com.dasi.ppt.agent.interfaces.controller.asset;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.service.AssetService;
import com.dasi.ppt.agent.interfaces.controller.IdVo;
import com.dasi.ppt.agent.interfaces.controller.asset.dto.*;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.AssetInfoVo;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.AssetListVo;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.TagListVo;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.TagListVo.TagInfoVo;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/assets")
@RequiredArgsConstructor
public class AssetController {

    private final AssetService assetService;

    @GetMapping
    public AssetListVo listAssets(@RequestParam String type, @RequestParam(required = false) String query) {
        return AssetListVo.builder()
                .items(assetService.listAssets(AssetType.fromValue(type), query)
                    .stream()
                    .map(AssetInfoVo::toVo)
                    .toList())
                .build();
    }

    @GetMapping("/{id}")
    public AssetInfoVo getAssetDetail(@PathVariable("id") String id) {
        return AssetInfoVo.toVo(assetService.getAssetDetail(id));
    }

    @PostMapping("/create")
    public IdVo createAsset(@Valid @RequestBody CreateAssetDto request) {
        return IdVo.builder()
                .id(assetService.createAsset(request.toEntity()))
                .build();
    }

    @PostMapping("/update")
    public void updateAsset(@Valid @RequestBody UpdateAssetDto request) {
        assetService.updateAsset(request.toEntity());
    }

    @PostMapping("/delete")
    public void deleteAsset(@Valid @RequestBody DeleteAssetDto request) {
        assetService.deleteAsset(request.getAssetId());
    }

    @GetMapping("/tags")
    public TagListVo listTags(@RequestParam String type) {
        return TagListVo.builder()
                .items(assetService.listTags(AssetType.fromValue(type))
                    .stream()
                    .map(TagInfoVo::toVo)
                    .toList())
                .build();
    }

    @PostMapping("/tags/create")
    public IdVo createTag(@Valid @RequestBody CreateTagDto request) {
        return IdVo.builder()
                .id(assetService.createTag(request.getType(), request.getName()))
                .build();
    }

    @PostMapping("/tags/update")
    public void updateTag(@Valid @RequestBody UpdateTagDto request) {
        assetService.updateTag(request.getTagId(), request.getName());
    }

    @PostMapping("/tags/delete")
    public void deleteTag(@Valid @RequestBody DeleteTagDto request) {
        assetService.deleteTag(request.getTagId());
    }
}
