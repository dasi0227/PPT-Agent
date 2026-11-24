package com.dasi.ppt.agent.interfaces.controller.setting;

import com.dasi.ppt.agent.domain.setting.service.SettingService;
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
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.nio.file.Path;

@RestController
@RequestMapping("/settings")
@RequiredArgsConstructor
public class SettingController {

    private final SettingService settingService;

    @PostMapping("/fileopen/pick")
    public PickApplicationVo pickApplication() {
        Path path = settingService.pickApplication();
        return PickApplicationVo.builder().path(path == null ? null : path.toString()).build();
    }

    @GetMapping("/fileopen")
    public FileOpenSettingsVo getFileOpen() {
        return FileOpenSettingsVo.toVo(settingService.getFileOpenSettings());
    }

    @PostMapping("/fileopen/save")
    public FileOpenSettingsVo saveFileOpen(@Valid @RequestBody SaveFileOpenSettingsDto request) {
        return FileOpenSettingsVo.toVo(settingService.saveFileOpenSettings(request.toFileOpenSettings()));
    }

    @GetMapping("/shortcuts")
    public ShortcutSettingsVo getShortcuts() {
        return ShortcutSettingsVo.toVo(settingService.getShortcuts());
    }

    @PostMapping("/shortcuts/save")
    public void saveShortcuts(@Valid @RequestBody SaveShortcutSettingsDto request) {
        settingService.saveShortcuts(request.toShortcutSettings());
    }

    @GetMapping("/models")
    public ModelSettingsVo getModelSettings() {
        return ModelSettingsVo.toVo(settingService.getModelSettings());
    }

    @PostMapping("/models/create")
    public ModelConfigVo createModel(@Valid @RequestBody CreateModelDto request) {
        return ModelConfigVo.toVo(settingService.createModel(request.toModelConfigSettings()));
    }

    @PostMapping("/models/update")
    public ModelConfigVo updateModel(@Valid @RequestBody UpdateModelDto request) {
        return ModelConfigVo.toVo(settingService.updateModel(request.toModelConfigSettings()));
    }

    @PostMapping("/models/delete")
    public void deleteModel(@Valid @RequestBody DeleteModelDto request) {
        settingService.deleteModel(request.getId());
    }

    @PostMapping("/routes/save")
    public void saveRoute(@Valid @RequestBody SaveRouteDto request) {
        settingService.saveRoute(request.getRoute(), request.getModelId());
    }

    @GetMapping("/behavior")
    public BehaviorSettingsVo getBehavior() {
        return BehaviorSettingsVo.toVo(settingService.getBehavior());
    }

    @PostMapping("/behavior/save")
    public void saveBehavior(@RequestBody SaveBehaviorSettingsDto request) {
        settingService.saveBehavior(request.toBehaviorSettings());
    }
}
