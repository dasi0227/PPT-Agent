package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.setting.model.entity.BehaviorSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelRouteSettings;
import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelProvider;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelRoute;
import com.dasi.ppt.agent.domain.setting.repository.ISettingRepository;
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
    public FileOpenSettings getFileOpenSettings() {
        return toEntity(fileOpenSettingsMapper.getFileOpenSettings());
    }

    @Override
    public void saveFileOpenSettings(FileOpenSettings settings) {
        fileOpenSettingsMapper.saveFileOpenSettings(toPo(settings));
    }

    @Override
    public ShortcutSettings getShortcutSettings() {
        return toEntity(shortcutSettingsMapper.getShortcutSettings());
    }

    @Override
    public void saveShortcutSettings(ShortcutSettings settings) {
        shortcutSettingsMapper.saveShortcutSettings(toPo(settings));
    }

    @Override
    public BehaviorSettings getBehaviorSettings() {
        return toEntity(behaviorSettingsMapper.getBehaviorSettings());
    }

    @Override
    public void saveBehaviorSettings(BehaviorSettings settings) {
        int updated = behaviorSettingsMapper.saveBehaviorSettings(toPo(settings));
        if (updated == 0) {
            throw new DataRetrievalFailureException("缺少 settings_behavior 初始化记录");
        }
    }

    @Override
    public List<ModelConfigSettings> listModels() {
        return llmConfigMapper.listModels()
                .stream()
                .map(SettingRepository::toEntity)
                .toList();
    }

    @Override
    public ModelConfigSettings findModelById(String id) {
        return toEntity(llmConfigMapper.findModelById(id));
    }

    @Override
    public ModelRouteSettings getModelRoutes() {
        return toEntity(routeSettingsMapper.getModelRoutes());
    }

    @Override
    public boolean existsConflictingModelName(String name, String modelId) {
        return llmConfigMapper.existsConflictingModelName(name, modelId);
    }

    @Override
    public boolean existsModelById(String id) {
        return llmConfigMapper.existsModelById(id);
    }

    @Override
    public boolean isLastModel(String id) {
        return llmConfigMapper.isLastModel(id);
    }

    @Override
    public void insertModel(ModelConfigSettings model) {
        llmConfigMapper.insertModel(toPo(model));
    }

    @Override
    public int updateModel(ModelConfigSettings model) {
        return llmConfigMapper.updateModel(toPo(model));
    }

    @Override
    public int deleteModel(String id) {
        return llmConfigMapper.deleteModel(id);
    }

    @Override
    public void saveModelRoute(ModelRoute route, String modelId) {
        if (routeSettingsMapper.saveModelRoute(route, modelId) == 0) {
            throw new DataRetrievalFailureException("缺少 settings_route 初始化记录");
        }
    }

    private FileOpenSettings toEntity(FileOpenSettingsPo po) {
        if (po == null) return null;
        List<String> apps = objectMapper.readValue(po.getCustomApps(), new TypeReference<>() {});
        return FileOpenSettings.builder()
                .defaultOpen(po.getDefaultOpen())
                .jsonOpen(po.getJsonOpen())
                .htmlOpen(po.getHtmlOpen())
                .customApps(apps)
                .build();
    }

    private FileOpenSettingsPo toPo(FileOpenSettings settings) {
        return FileOpenSettingsPo.builder()
                .id(1)
                .defaultOpen(settings.getDefaultOpen())
                .jsonOpen(settings.getJsonOpen())
                .htmlOpen(settings.getHtmlOpen())
                .customApps(objectMapper.writeValueAsString(settings.getCustomApps()))
                .build();
    }

    private static ShortcutSettings toEntity(ShortcutSettingsPo po) {
        if (po == null) return null;
        return ShortcutSettings.builder()
                .commandTrigger(po.getCommandTrigger())
                .referenceTrigger(po.getReferenceTrigger())
                .componentTrigger(po.getComponentTrigger())
                .slideTrigger(po.getSlideTrigger())
                .snippetTrigger(po.getSnippetTrigger())
                .previousSlide(po.getPreviousSlide())
                .nextSlide(po.getNextSlide())
                .zoomIn(po.getZoomIn())
                .zoomOut(po.getZoomOut())
                .toggleOverview(po.getToggleOverview())
                .exportDeck(po.getExportDeck())
                .toggleView(po.getToggleView())
                .toggleSource(po.getToggleSource())
                .switchTheme(po.getSwitchTheme())
                .submitRequest(po.getSubmitRequest())
                .selectSuggestion(po.getSelectSuggestion())
                .build();
    }

    private static ShortcutSettingsPo toPo(ShortcutSettings settings) {
        return ShortcutSettingsPo.builder()
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
                .build();
    }

    private static BehaviorSettings toEntity(BehaviorSettingsPo po) {
        if (po == null) return null;
        return BehaviorSettings.builder()
                .autoRenameEnabled(po.getAutoRenameEnabled())
                .autoSuggestEnabled(po.getAutoSuggestEnabled())
                .showToolFailures(po.getShowToolFailures())
                .requireResourceApproval(po.getRequireResourceApproval())
                .requireHighRiskApproval(po.getRequireHighRiskApproval())
                .build();
    }

    private static BehaviorSettingsPo toPo(BehaviorSettings settings) {
        return BehaviorSettingsPo.builder()
                .id(1)
                .autoRenameEnabled(settings.getAutoRenameEnabled())
                .autoSuggestEnabled(settings.getAutoSuggestEnabled())
                .showToolFailures(settings.getShowToolFailures())
                .requireResourceApproval(settings.getRequireResourceApproval())
                .requireHighRiskApproval(settings.getRequireHighRiskApproval())
                .build();
    }

    private static ModelRouteSettings toEntity(RouteSettingsPo po) {
        if (po == null) return null;
        return ModelRouteSettings.builder()
                .mainDefault(po.getMainDefault())
                .mainFallback(po.getMainFallback())
                .sideDefault(po.getSideDefault())
                .sideFallback(po.getSideFallback())
                .rename(po.getRename())
                .compact(po.getCompact())
                .commit(po.getCommit())
                .polish(po.getPolish())
                .handoff(po.getHandoff())
                .suggest(po.getSuggest())
                .build();
    }

    private static ModelConfigSettings toEntity(LlmConfigPo po) {
        if (po == null) return null;
        try {
            return ModelConfigSettings.builder()
                    .id(po.getId())
                    .name(po.getName())
                    .protocol(LlmProtocol.fromValue(po.getProtocol()))
                    .provider(ModelProvider.fromValue(po.getProvider()))
                    .baseUrl(po.getBaseUrl())
                    .model(po.getModel())
                    .build();
        } catch (InvalidEnumValueException ex) {
            throw new DataRetrievalFailureException("settings_models 包含无效的枚举值", ex);
        }
    }

    private static LlmConfigPo toPo(ModelConfigSettings model) {
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
