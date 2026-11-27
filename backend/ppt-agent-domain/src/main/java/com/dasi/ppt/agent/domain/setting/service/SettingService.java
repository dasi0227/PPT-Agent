package com.dasi.ppt.agent.domain.setting.service;

import com.dasi.ppt.agent.domain.setting.model.entity.*;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelRoute;
import com.dasi.ppt.agent.domain.setting.repository.ISettingRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.nio.file.InvalidPathException;
import java.nio.file.Path;
import java.util.*;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.*;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Service
@RequiredArgsConstructor
public class SettingService {

    private final ISettingRepository settingRepository;

    private static final Pattern SHORTCUT = Pattern.compile("(Primary|Option|Alt)\\+([^+\\s]+)", Pattern.UNICODE_CHARACTER_CLASS);
    private static final Set<String> MODIFIER_KEYS = Set.of("Primary", "Option", "Alt", "AltLeft", "AltRight", "AltGraph", "Shift", "ShiftLeft", "ShiftRight", "Ctrl", "Control", "ControlLeft", "ControlRight", "Command", "Meta", "MetaLeft", "MetaRight", "OSLeft", "OSRight", "Fn");
    private static final Set<String> RESERVED_SHORTCUTS = Set.of("Primary+KeyA", "Primary+KeyC", "Primary+KeyV", "Primary+KeyX", "Primary+KeyZ", "Primary+KeyY", "Primary+KeyF", "Primary+KeyL", "Primary+KeyQ", "Primary+KeyW", "Primary+KeyR", "Primary+KeyN");

    public FileOpenSettings getFileOpenSettings() {
        return settingRepository.getFileOpenSettings();
    }

    @Transactional
    public FileOpenSettings saveFileOpenSettings(FileOpenSettings settings) {
        Map<Path, String> apps = new LinkedHashMap<>();
        for (String value : settings.getCustomApps()) {
            Path path = applicationPath(value);
            apps.putIfAbsent(path, path.toString());
        }
        settings.setCustomApps(List.copyOf(apps.values()));
        settings.setDefaultOpen(resolveOpen(settings.getDefaultOpen(), apps, SYSTEM_OPEN));
        settings.setJsonOpen(resolveOpen(settings.getJsonOpen(), apps, INHERIT_OPEN));
        settings.setHtmlOpen(resolveOpen(settings.getHtmlOpen(), apps, INHERIT_OPEN));
        settingRepository.saveFileOpenSettings(settings);
        return settings;
    }

    private static String resolveOpen(String value, Map<Path, String> apps, String fallback) {
        if (SYSTEM_OPEN.equals(value) || fallback.equals(value)) {
            return value;
        }
        return apps.getOrDefault(applicationPath(value), fallback);
    }

    private static Path applicationPath(String value) {
        try {
            Path path = Path.of(value).normalize();
            String normalized = path.toString().toLowerCase(Locale.ROOT);
            if (path.isAbsolute() && normalized.endsWith(APPLICATION_SUFFIX)) {
                return path;
            }
        } catch (InvalidPathException ex) {
            throw invalid("应用路径无效");
        }
        throw invalid("应用需使用以 " + APPLICATION_SUFFIX + " 结尾的绝对路径");
    }

    public ShortcutSettings getShortcutSettings() {
        return settingRepository.getShortcutSettings();
    }

    @Transactional
    public void saveShortcutSettings(ShortcutSettings settings) {
        List<String> triggers = List.of(
                settings.getCommandTrigger(), settings.getReferenceTrigger(),
                settings.getComponentTrigger(), settings.getSlideTrigger(),
                settings.getSnippetTrigger()
        );
        if (new HashSet<>(triggers).size() != triggers.size()) {
            throw invalid("输入触发符不能重复");
        }

        String suggestion = settings.getSelectSuggestion();
        List<String> shortcuts = List.of(
                settings.getPreviousSlide(), settings.getNextSlide(), settings.getZoomIn(),
                settings.getZoomOut(), settings.getToggleOverview(), settings.getExportDeck(), settings.getToggleView(),
                settings.getToggleSource(), settings.getSwitchTheme(), settings.getSubmitRequest(),
                suggestion + "+Digit1", suggestion + "+Digit2", suggestion + "+Digit3"
        );
        Set<String> assigned = new HashSet<>();
        for (String shortcut : shortcuts) {
            Matcher binding = SHORTCUT.matcher(shortcut);
            if (!binding.matches() || MODIFIER_KEYS.contains(binding.group(2))) {
                throw invalid("快捷键需由一个修饰键和一个普通键组成：" + shortcut);
            }
            if (RESERVED_SHORTCUTS.contains(shortcut)) {
                throw invalid("该快捷键已由系统保留：" + shortcut);
            }
            String signature = "Option".equals(binding.group(1)) ? "Alt+" + binding.group(2) : shortcut;
            if (!assigned.add(signature)) {
                throw invalid("快捷键重复：" + shortcut);
            }
        }
        settingRepository.saveShortcutSettings(settings);
    }

    @Transactional(readOnly = true)
    public ModelSettings getModelSettings() {
        return ModelSettings.builder()
                .models(settingRepository.listModels())
                .routes(settingRepository.getModelRoutes())
                .build();
    }

    public ModelConfigSettings getModelDetail(String modelId) {
        ModelConfigSettings model = settingRepository.findModelById(modelId);
        if (model == null) {
            throw notFound(ErrorCode.MODEL_NOT_FOUND);
        }
        return model;
    }

    @Transactional
    public String createModel(ModelConfigSettings config) {
        if (settingRepository.existsModelName(config.getName(), null)) {
            throw invalid("模型配置名不能重复");
        }
        config.setId(IdUtil.genModelId());
        settingRepository.insertModel(config);
        return config.getId();
    }

    @Transactional
    public void updateModel(ModelConfigSettings config) {
        if (config.getName() != null && settingRepository.existsModelName(config.getName(), config.getId())) {
            throw invalid("模型配置名不能重复");
        }
        if (settingRepository.updateModel(config) == 0) {
            throw notFound(ErrorCode.MODEL_NOT_FOUND);
        }
    }

    @Transactional
    public void deleteModel(String modelId) {
        if (settingRepository.isLastModel(modelId)) {
            throw invalid("至少保留一个模型配置");
        }
        if (settingRepository.getModelRoutes().usesModel(modelId)) {
            throw conflict(ErrorCode.MODEL_IN_USE);
        }
        if (settingRepository.deleteModel(modelId) == 0) {
            throw notFound(ErrorCode.MODEL_NOT_FOUND);
        }
    }

    @Transactional
    public void saveModelRoute(ModelRoute route, String modelId) {
        if (modelId == null && (route == ModelRoute.MAIN_DEFAULT || route == ModelRoute.SIDE_DEFAULT)) {
            throw invalid("默认路由需指定模型配置 ID");
        }
        if (modelId != null && !settingRepository.existsById(modelId)) {
            throw notFound(ErrorCode.MODEL_NOT_FOUND);
        }
        settingRepository.saveModelRoute(route, modelId);
    }

    public BehaviorSettings getBehaviorSettings() {
        return settingRepository.getBehaviorSettings();
    }

    @Transactional
    public void saveBehaviorSettings(BehaviorSettings settings) {
        settingRepository.saveBehaviorSettings(settings);
    }
}
