package com.dasi.ppt.agent.interfaces.controller.asset;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.service.AssetService;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import com.dasi.ppt.agent.interfaces.controller.asset.dto.CreateAssetDto;
import com.dasi.ppt.agent.interfaces.controller.asset.dto.CreateTagDto;
import com.dasi.ppt.agent.interfaces.controller.asset.dto.DeleteAssetDto;
import com.dasi.ppt.agent.interfaces.controller.asset.dto.DeleteTagDto;
import com.dasi.ppt.agent.interfaces.controller.asset.dto.UpdateAssetDto;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.AssetContentVo;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.AssetInfoVo;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.AssetListVo;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.TagInfoVo;
import com.dasi.ppt.agent.interfaces.controller.asset.vo.TagListVo;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/assets")
@RequiredArgsConstructor
public class AssetController {

    private final AssetService assetService;

    @GetMapping
    public AssetListVo list(@RequestParam String type, @RequestParam(required = false) String query) {
        List<AssetInfoVo> items = assetService.listAssets(AssetType.fromValue(type), query)
                .stream()
                .map(AssetInfoVo::toVo)
                .toList();
        return AssetListVo.builder().items(items).build();
    }

    @GetMapping("/{id}")
    public AssetContentVo content(@PathVariable("id") String id) {
        Asset asset = assetService.findAsset(id);
        return AssetContentVo.builder()
                .content(asset.getContent())
                .hash(HashUtil.calAssetHash(asset))
                .build();
    }

    @GetMapping("/tags")
    public TagListVo tags(@RequestParam String type) {
        List<TagInfoVo> items = assetService.listTags(AssetType.fromValue(type))
                .stream()
                .map(TagInfoVo::toVo)
                .toList();
        return TagListVo.builder().items(items).build();
    }

    @PostMapping("/tags/create")
    public TagInfoVo createTag(@Valid @RequestBody CreateTagDto request) {
        return TagInfoVo.toVo(assetService.createTag(request.getType(), request.getName()));
    }

    @PostMapping("/tags/delete")
    public void deleteTag(@Valid @RequestBody DeleteTagDto request) {
        assetService.deleteTag(request.getTagId());
    }

    @PostMapping("/create")
    public AssetInfoVo create(@Valid @RequestBody CreateAssetDto request) {
        return AssetInfoVo.toVo(assetService.createAsset(request.toAsset()));
    }

    @PostMapping("/update")
    public AssetInfoVo update(@Valid @RequestBody UpdateAssetDto request) {
        return AssetInfoVo.toVo(assetService.updateAsset(request.toAsset(), request.getBaseHash()));
    }

    @PostMapping("/delete")
    public void delete(@Valid @RequestBody DeleteAssetDto request) {
        assetService.delete(request.getAssetId());
    }
}
