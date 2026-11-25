package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.setting.model.entity.BehaviorSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelRouteSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelProvider;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelRoute;
import com.dasi.ppt.agent.domain.setting.repository.ISettingRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.BehaviorSettingsMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.FileOpenSettingsMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.LlmConfigMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.RouteSettingsMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ShortcutSettingsMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.BehaviorSettingsPo;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.FileOpenSettingsPo;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.LlmConfigPo;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.RouteSettingsPo;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.ShortcutSettingsPo;
import lombok.RequiredArgsConstructor;
import org.springframework.dao.DataRetrievalFailureException;
import org.springframework.stereotype.Repository;
import tools.jackson.core.type.TypeReference;
import tools.jackson.databind.ObjectMapper;

import java.util.List;
import java.util.Objects;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Repository
@RequiredArgsConstructor
public class SettingRepository implements ISettingRepository {

    private final FileOpenSettingsMapper fileOpenSettingsMapper;
    private final ShortcutSettingsMapper shortcutSettingsMapper;
    private final LlmConfigMapper llmConfigMapper;
    private final RouteSettingsMapper routeSettingsMapper;
    private final BehaviorSettingsMapper behaviorSettingsMapper;
    private final ObjectMapper objectMapper;

    @Override
    public FileOpenSettings readFileOpenSettings() {
        FileOpenSettingsPo row = Objects.requireNonNull(fileOpenSettingsMapper.read(), "缺少 settings_fileopen 初始化记录");
        List<String> apps = objectMapper.readValue(row.getCustomApps(), new TypeReference<>() {});
        return FileOpenSettings.builder()
                .defaultOpen(row.getDefaultOpen())
                .jsonOpen(row.getJsonOpen())
                .htmlOpen(row.getHtmlOpen())
                .customApps(apps)
                .build();
    }

    @Override
    public void saveFileOpenSettings(FileOpenSettings settings) {
        fileOpenSettingsMapper.save(FileOpenSettingsPo.builder()
                .id(1)
                .defaultOpen(settings.getDefaultOpen())
                .jsonOpen(settings.getJsonOpen())
                .htmlOpen(settings.getHtmlOpen())
                .customApps(objectMapper.writeValueAsString(settings.getCustomApps()))
                .build());
    }

    @Override
    public ShortcutSettings readShortcuts() {
        ShortcutSettingsPo row = Objects.requireNonNull(shortcutSettingsMapper.read(), "缺少 settings_shortcut 初始化记录");
        return ShortcutSettings.builder()
                .commandTrigger(row.getCommandTrigger())
                .referenceTrigger(row.getReferenceTrigger())
                .componentTrigger(row.getComponentTrigger())
                .slideTrigger(row.getSlideTrigger())
                .snippetTrigger(row.getSnippetTrigger())
                .previousSlide(row.getPreviousSlide())
                .nextSlide(row.getNextSlide())
                .zoomIn(row.getZoomIn())
                .zoomOut(row.getZoomOut())
                .toggleOverview(row.getToggleOverview())
                .exportDeck(row.getExportDeck())
                .toggleView(row.getToggleView())
                .toggleSource(row.getToggleSource())
                .switchTheme(row.getSwitchTheme())
                .submitRequest(row.getSubmitRequest())
                .selectSuggestion(row.getSelectSuggestion())
                .build();
    }

    @Override
    public void saveShortcuts(ShortcutSettings settings) {
        shortcutSettingsMapper.save(ShortcutSettingsPo.builder()
                .id(1)
                .commandTrigger(settings.getCommandTrigger())
                .referenceTrigger(settings.getReferenceTrigger())
                .componentTrigger(settings.getComponentTrigger())
                .slideTrigger(settings.getSlideTrigger())
                .snippetTrigger(settings.getSnippetTrigger())
                .previousSlide(settings.getPreviousSlide())
                .nextSlide(settings.getNextSlide())
                .zoomIn(settings.getZoomIn())
                .zoomOut(settings.getZoomOut())
                .toggleOverview(settings.getToggleOverview())
                .exportDeck(settings.getExportDeck())
                .toggleView(settings.getToggleView())
                .toggleSource(settings.getToggleSource())
                .switchTheme(settings.getSwitchTheme())
                .submitRequest(settings.getSubmitRequest())
                .selectSuggestion(settings.getSelectSuggestion())
                .build());
    }

    @Override
    public BehaviorSettings readBehaviorSettings() {
        BehaviorSettingsPo row = Objects.requireNonNull(behaviorSettingsMapper.read(), "缺少 settings_behavior 初始化记录");
        return BehaviorSettings.builder()
                .autoRenameEnabled(row.getAutoRenameEnabled())
                .autoSuggestEnabled(row.getAutoSuggestEnabled())
                .showToolFailures(row.getShowToolFailures())
                .requireResourceApproval(row.getRequireResourceApproval())
                .requireHighRiskApproval(row.getRequireHighRiskApproval())
                .build();
    }

    @Override
    public void saveBehaviorSettings(BehaviorSettings settings) {
        int updated = behaviorSettingsMapper.save(BehaviorSettingsPo.builder()
                .id(1)
                .autoRenameEnabled(settings.getAutoRenameEnabled())
                .autoSuggestEnabled(settings.getAutoSuggestEnabled())
                .showToolFailures(settings.getShowToolFailures())
                .requireResourceApproval(settings.getRequireResourceApproval())
                .requireHighRiskApproval(settings.getRequireHighRiskApproval())
                .build());
        if (updated == 0) {
            throw new DataRetrievalFailureException("缺少 settings_behavior 初始化记录");
        }
    }

    @Override
    public List<ModelConfigSettings> readModels() {
        return llmConfigMapper.list()
                .stream()
                .map(this::toModelConfigSettings)
                .toList();
    }

    @Override
    public ModelConfigSettings readModel(String id) {
        LlmConfigPo row = llmConfigMapper.find(id);
        if (row == null) {
            throw notFound(ErrorCode.MODEL_NOT_FOUND);
        }
        return toModelConfigSettings(row);
    }

    @Override
    public ModelRouteSettings readRoutes() {
        RouteSettingsPo row = Objects.requireNonNull(routeSettingsMapper.read(), "缺少 settings_route 初始化记录");
        return ModelRouteSettings.builder()
                .mainDefault(row.getMainDefault())
                .mainFallback(row.getMainFallback())
                .sideDefault(row.getSideDefault())
                .sideFallback(row.getSideFallback())
                .rename(row.getRename())
                .compact(row.getCompact())
                .commit(row.getCommit())
                .polish(row.getPolish())
                .handoff(row.getHandoff())
                .suggest(row.getSuggest())
                .build();
    }

    @Override
    public boolean modelNameExists(String name, String excludeId) {
        return llmConfigMapper.nameExists(name, excludeId);
    }

    @Override
    public boolean modelExists(String id) {
        return llmConfigMapper.exists(id);
    }

    @Override
    public int countModels() {
        return llmConfigMapper.count();
    }

    @Override
    public void insertModel(ModelConfigSettings model) {
        llmConfigMapper.insert(toPo(model));
    }

    @Override
    public void updateModel(ModelConfigSettings model) {
        if (llmConfigMapper.update(toPo(model)) == 0) {
            throw notFound(ErrorCode.MODEL_NOT_FOUND);
        }
    }

    @Override
    public void deleteModel(String id) {
        llmConfigMapper.delete(id);
    }

    @Override
    public void saveRoute(ModelRoute route, String modelId) {
        if (routeSettingsMapper.updateRoute(route, modelId) == 0) {
            throw new DataRetrievalFailureException("缺少 settings_route 初始化记录");
        }
    }

    private ModelConfigSettings toModelConfigSettings(LlmConfigPo row) {
        try {
            return ModelConfigSettings.builder()
                    .id(row.getId())
                    .name(row.getName())
                    .protocol(LlmProtocol.fromValue(row.getProtocol()))
                    .provider(ModelProvider.fromValue(row.getProvider()))
                    .baseUrl(row.getBaseUrl())
                    .model(row.getModel())
                    .build();
        } catch (InvalidEnumValueException ex) {
            throw new DataRetrievalFailureException("settings_models 包含无效的枚举值", ex);
        }
    }

    private LlmConfigPo toPo(ModelConfigSettings model) {
        return LlmConfigPo.builder()
                .id(model.getId())
                .name(model.getName())
                .protocol(model.getProtocol() == null ? null : model.getProtocol().getValue())
                .provider(model.getProvider() == null ? null : model.getProvider().getValue())
                .baseUrl(model.getBaseUrl())
                .model(model.getModel())
                .key(model.getKey())
                .build();
    }
}
