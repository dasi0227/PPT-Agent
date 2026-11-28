package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.Getter;

@Getter
public final class SaveShortcutSettingsDto {
    private static final String TRIGGER = "[/~!@#$%^&*>\\\\+]";

    @NotNull(message = "command_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "command_trigger 格式无效")
    private String commandTrigger;
    @NotNull(message = "reference_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "reference_trigger 格式无效")
    private String referenceTrigger;
    @NotNull(message = "component_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "component_trigger 格式无效")
    private String componentTrigger;
    @NotNull(message = "slide_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "slide_trigger 格式无效")
    private String slideTrigger;
    @NotNull(message = "snippet_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "snippet_trigger 格式无效")
    private String snippetTrigger;
    @NotBlank(message = "previous_slide 不能为空")
    private String previousSlide;
    @NotBlank(message = "next_slide 不能为空")
    private String nextSlide;
    @NotBlank(message = "zoom_in 不能为空")
    private String zoomIn;
    @NotBlank(message = "zoom_out 不能为空")
    private String zoomOut;
    @NotBlank(message = "toggle_overview 不能为空")
    private String toggleOverview;
    @NotBlank(message = "export_deck 不能为空")
    private String exportDeck;
    @NotBlank(message = "toggle_view 不能为空")
    private String toggleView;
    @NotBlank(message = "toggle_source 不能为空")
    private String toggleSource;
    @NotBlank(message = "switch_theme 不能为空")
    private String switchTheme;
    @NotBlank(message = "submit_request 不能为空")
    private String submitRequest;
    @NotBlank(message = "select_suggestion 不能为空")
    private String selectSuggestion;

    public ShortcutSettings toEntity() {
        return ShortcutSettings.builder()
                .commandTrigger(commandTrigger)
                .referenceTrigger(referenceTrigger)
                .componentTrigger(componentTrigger)
                .slideTrigger(slideTrigger)
                .snippetTrigger(snippetTrigger)
                .previousSlide(previousSlide)
                .nextSlide(nextSlide)
                .zoomIn(zoomIn)
                .zoomOut(zoomOut)
                .toggleOverview(toggleOverview)
                .exportDeck(exportDeck)
                .toggleView(toggleView)
                .toggleSource(toggleSource)
                .switchTheme(switchTheme)
                .submitRequest(submitRequest)
                .selectSuggestion(selectSuggestion)
                .build();
    }
}
