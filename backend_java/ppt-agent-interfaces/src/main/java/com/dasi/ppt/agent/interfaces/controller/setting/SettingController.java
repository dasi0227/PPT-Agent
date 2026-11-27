package com.dasi.ppt.agent.interfaces.controller.setting;

import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.gateway.IPlatformGateway;
import com.dasi.ppt.agent.interfaces.controller.IdVo;
import com.dasi.ppt.agent.interfaces.controller.setting.dto.CreateModelDto;
import com.dasi.ppt.agent.interfaces.controller.setting.dto.DeleteModelDto;
import com.dasi.ppt.agent.interfaces.controller.setting.dto.SaveBehaviorSettingsDto;
import com.dasi.ppt.agent.interfaces.controller.setting.dto.SaveFileOpenSettingsDto;
import com.dasi.ppt.agent.interfaces.controller.setting.dto.SaveRouteDto;
import com.dasi.ppt.agent.interfaces.controller.setting.dto.SaveShortcutSettingsDto;
import com.dasi.ppt.agent.interfaces.controller.setting.dto.UpdateModelDto;
import com.dasi.ppt.agent.interfaces.controller.setting.vo.BehaviorSettingsVo;
import com.dasi.ppt.agent.interfaces.controller.setting.vo.FileOpenSettingsVo;
import com.dasi.ppt.agent.interfaces.controller.setting.vo.ModelConfigVo;
import com.dasi.ppt.agent.interfaces.controller.setting.vo.ModelSettingsVo;
import com.dasi.ppt.agent.interfaces.controller.setting.vo.PickApplicationVo;
import com.dasi.ppt.agent.interfaces.controller.setting.vo.ShortcutSettingsVo;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/settings")
@RequiredArgsConstructor
public class SettingController {

    private final SettingService settingService;
    private final IPlatformGateway platformGateway;

    @GetMapping("/fileopen")
    public FileOpenSettingsVo getFileOpenSettings() {
        return FileOpenSettingsVo.toVo(settingService.getFileOpenSettings());
    }

    @PostMapping("/fileopen/save")
    public FileOpenSettingsVo saveFileOpenSettings(@Valid @RequestBody SaveFileOpenSettingsDto request) {
        return FileOpenSettingsVo.toVo(settingService.saveFileOpenSettings(request.toEntity()));
    }

    @PostMapping("/fileopen/pick")
    public PickApplicationVo pickApplication() {
        return PickApplicationVo.toVo(platformGateway.pickApp());
    }

    @GetMapping("/shortcuts")
    public ShortcutSettingsVo getShortcutSettings() {
        return ShortcutSettingsVo.toVo(settingService.getShortcutSettings());
    }

    @PostMapping("/shortcuts/save")
    public void saveShortcutSettings(@Valid @RequestBody SaveShortcutSettingsDto request) {
        settingService.saveShortcutSettings(request.toEntity());
    }

    @GetMapping("/models")
    public ModelSettingsVo getModelSettings() {
        return ModelSettingsVo.toVo(settingService.getModelSettings());
    }

    @GetMapping("/models/{id}")
    public ModelConfigVo getModelDetail(@PathVariable("id") String modelId) {
        return ModelConfigVo.toVo(settingService.getModelDetail(modelId));
    }

    @PostMapping("/models/create")
    public IdVo createModel(@Valid @RequestBody CreateModelDto request) {
        return IdVo.builder()
                .id(settingService.createModel(request.toEntity()))
                .build();
    }

    @PostMapping("/models/update")
    public void updateModel(@Valid @RequestBody UpdateModelDto request) {
        settingService.updateModel(request.toEntity());
    }

    @PostMapping("/models/delete")
    public void deleteModel(@Valid @RequestBody DeleteModelDto request) {
        settingService.deleteModel(request.getId());
    }

    @PostMapping("/routes/save")
    public void saveModelRoute(@Valid @RequestBody SaveRouteDto request) {
        settingService.saveModelRoute(request.getRoute(), request.getModelId());
    }

    @GetMapping("/behavior")
    public BehaviorSettingsVo getBehaviorSettings() {
        return BehaviorSettingsVo.toVo(settingService.getBehaviorSettings());
    }

    @PostMapping("/behavior/save")
    public void saveBehaviorSettings(@Valid @RequestBody SaveBehaviorSettingsDto request) {
        settingService.saveBehaviorSettings(request.toEntity());
    }
}
