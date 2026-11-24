package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class SaveShortcutSettingsDto {
    @NotBlank(message = "command_trigger 不能为空白")
    private String commandTrigger;
    @NotBlank(message = "reference_trigger 不能为空白")
    private String referenceTrigger;
    @NotBlank(message = "component_trigger 不能为空白")
    private String componentTrigger;
    @NotBlank(message = "slide_trigger 不能为空白")
    private String slideTrigger;
    @NotBlank(message = "snippet_trigger 不能为空白")
    private String snippetTrigger;
    @NotBlank(message = "previous_slide 不能为空白")
    private String previousSlide;
    @NotBlank(message = "next_slide 不能为空白")
    private String nextSlide;
    @NotBlank(message = "zoom_in 不能为空白")
    private String zoomIn;
    @NotBlank(message = "zoom_out 不能为空白")
    private String zoomOut;
    @NotBlank(message = "toggle_overview 不能为空白")
    private String toggleOverview;
    @NotBlank(message = "export_deck 不能为空白")
    private String exportDeck;
    @NotBlank(message = "toggle_view 不能为空白")
    private String toggleView;
    @NotBlank(message = "toggle_source 不能为空白")
    private String toggleSource;
    @NotBlank(message = "switch_theme 不能为空白")
    private String switchTheme;
    @NotBlank(message = "submit_request 不能为空白")
    private String submitRequest;
    @NotBlank(message = "select_suggestion 不能为空白")
    private String selectSuggestion;

    public ShortcutSettings toShortcutSettings() {
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
